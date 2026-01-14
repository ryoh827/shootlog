package exif

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	ifdTagMake             = 0x010F
	ifdTagModel            = 0x0110
	ifdTagOrientation      = 0x0112
	ifdTagSoftware         = 0x0131
	ifdTagDateTime         = 0x0132
	ifdTagExifOffset       = 0x8769
	ifdTagExposureTime     = 0x829A
	ifdTagFNumber          = 0x829D
	ifdTagISOSpeed         = 0x8827
	ifdTagDateTimeOriginal = 0x9003
	ifdTagExposureProgram  = 0x8822
	ifdTagMeteringMode     = 0x9207
	ifdTagFlash            = 0x9209
	ifdTagFocalLength      = 0x920A
	ifdTagWhiteBalance     = 0xA403
	ifdTagSceneCaptureType = 0xA406
	ifdTagLensModel        = 0xA434
)

const (
	typeByte     = 1
	typeASCII    = 2
	typeShort    = 3
	typeLong     = 4
	typeRational = 5
)

func parseEXIF(data []byte) (Summary, error) {
	segmentStart, err := findExifSegment(data)
	if err != nil {
		return Summary{}, err
	}

	if len(data) < segmentStart+6 {
		return Summary{}, ErrInvalidExif
	}

	if string(data[segmentStart:segmentStart+6]) != "Exif\x00\x00" {
		return Summary{}, ErrInvalidExif
	}

	tiffStart := segmentStart + 6
	if len(data) < tiffStart+8 {
		return Summary{}, ErrInvalidExif
	}

	order, err := byteOrder(data[tiffStart:])
	if err != nil {
		return Summary{}, err
	}

	ifdOffset := int(order.Uint32(data[tiffStart+4:]))
	if ifdOffset <= 0 {
		return Summary{}, ErrInvalidExif
	}

	ifd0Offset := tiffStart + ifdOffset
	tagValues, exifOffset, err := parseIFD(data, tiffStart, ifd0Offset, order)
	if err != nil {
		return Summary{}, err
	}

	if exifOffset > 0 {
		exifIFDOffset := tiffStart + exifOffset
		exifValues, _, err := parseIFD(data, tiffStart, exifIFDOffset, order)
		if err != nil {
			return Summary{}, err
		}
		for key, value := range exifValues {
			tagValues[key] = value
		}
	}

	summary := Summary{
		Make:             tagValues[ifdTagMake],
		Model:            tagValues[ifdTagModel],
		LensModel:        tagValues[ifdTagLensModel],
		DateTime:         firstNonEmpty(tagValues[ifdTagDateTimeOriginal], tagValues[ifdTagDateTime]),
		FNumber:          tagValues[ifdTagFNumber],
		ExposureTime:     tagValues[ifdTagExposureTime],
		ISOSpeed:         tagValues[ifdTagISOSpeed],
		FocalLength:      tagValues[ifdTagFocalLength],
		ExposureProgram:  tagValues[ifdTagExposureProgram],
		MeteringMode:     tagValues[ifdTagMeteringMode],
		WhiteBalance:     tagValues[ifdTagWhiteBalance],
		Software:         tagValues[ifdTagSoftware],
		Orientation:      tagValues[ifdTagOrientation],
		Flash:            tagValues[ifdTagFlash],
		SceneCaptureType: tagValues[ifdTagSceneCaptureType],
	}

	return summary, nil
}

func findExifSegment(data []byte) (int, error) {
	for i := 0; i+4 < len(data); i++ {
		if data[i] != 0xFF {
			continue
		}
		marker := data[i+1]
		if marker == 0xE1 {
			length := int(binary.BigEndian.Uint16(data[i+2:]))
			segmentStart := i + 4
			if length < 2 || segmentStart+length-2 > len(data) {
				return 0, ErrInvalidExif
			}
			return segmentStart, nil
		}
	}
	return 0, ErrExifNotFound
}

func byteOrder(data []byte) (binary.ByteOrder, error) {
	if len(data) < 2 {
		return nil, ErrInvalidExif
	}
	switch string(data[:2]) {
	case "II":
		return binary.LittleEndian, nil
	case "MM":
		return binary.BigEndian, nil
	default:
		return nil, ErrInvalidExif
	}
}

func parseIFD(data []byte, tiffStart, offset int, order binary.ByteOrder) (map[uint16]string, int, error) {
	if offset+2 > len(data) {
		return nil, 0, ErrInvalidExif
	}

	count := int(order.Uint16(data[offset:]))
	entryStart := offset + 2
	entrySize := 12
	entriesEnd := entryStart + count*entrySize
	if entriesEnd > len(data) {
		return nil, 0, ErrInvalidExif
	}

	values := make(map[uint16]string)
	exifOffset := 0

	for i := 0; i < count; i++ {
		entryOffset := entryStart + i*entrySize
		tag := order.Uint16(data[entryOffset:])
		fieldType := order.Uint16(data[entryOffset+2:])
		count := order.Uint32(data[entryOffset+4:])
		valueOffset := entryOffset + 8

		if tag == ifdTagExifOffset {
			if fieldType != typeLong || count != 1 {
				continue
			}
			exifOffset = int(order.Uint32(data[valueOffset:]))
			continue
		}

		value, ok := readIFDValue(data, tiffStart, fieldType, count, valueOffset, order)
		if !ok {
			continue
		}
		values[tag] = value
	}

	return values, exifOffset, nil
}

func readIFDValue(data []byte, tiffStart int, fieldType uint16, count uint32, valueOffset int, order binary.ByteOrder) (string, bool) {
	sizePerValue, ok := typeSize(fieldType)
	if !ok {
		return "", false
	}

	byteCount := int(count) * sizePerValue
	valueStart := valueOffset
	if byteCount > 4 {
		if valueOffset+4 > len(data) {
			return "", false
		}
		valueStart = tiffStart + int(order.Uint32(data[valueOffset:]))
	}

	if valueStart < 0 || valueStart+byteCount > len(data) {
		return "", false
	}

	valueData := data[valueStart : valueStart+byteCount]
	switch fieldType {
	case typeASCII:
		return strings.TrimRight(string(valueData), "\x00"), true
	case typeShort:
		if count == 1 {
			return fmt.Sprintf("%d", order.Uint16(valueData)), true
		}
		return formatShorts(valueData, order), true
	case typeLong:
		if count == 1 {
			return fmt.Sprintf("%d", order.Uint32(valueData)), true
		}
		return formatLongs(valueData, order), true
	case typeRational:
		return formatRationals(valueData, order), true
	case typeByte:
		if count == 1 {
			return fmt.Sprintf("%d", valueData[0]), true
		}
	}

	return "", false
}

func typeSize(fieldType uint16) (int, bool) {
	switch fieldType {
	case typeByte, typeASCII:
		return 1, true
	case typeShort:
		return 2, true
	case typeLong, typeRational:
		return 4, true
	default:
		return 0, false
	}
}

func formatShorts(data []byte, order binary.ByteOrder) string {
	parts := make([]string, 0, len(data)/2)
	for i := 0; i+2 <= len(data); i += 2 {
		parts = append(parts, fmt.Sprintf("%d", order.Uint16(data[i:])))
	}
	return strings.Join(parts, ",")
}

func formatLongs(data []byte, order binary.ByteOrder) string {
	parts := make([]string, 0, len(data)/4)
	for i := 0; i+4 <= len(data); i += 4 {
		parts = append(parts, fmt.Sprintf("%d", order.Uint32(data[i:])))
	}
	return strings.Join(parts, ",")
}

func formatRationals(data []byte, order binary.ByteOrder) string {
	parts := make([]string, 0, len(data)/8)
	for i := 0; i+8 <= len(data); i += 8 {
		numerator := order.Uint32(data[i:])
		denominator := order.Uint32(data[i+4:])
		if denominator == 0 {
			parts = append(parts, "0")
			continue
		}
		parts = append(parts, fmt.Sprintf("%d/%d", numerator, denominator))
	}
	return strings.Join(parts, ",")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
