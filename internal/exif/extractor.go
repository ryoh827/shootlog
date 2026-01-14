package exif

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	tagMake          = 0x010F
	tagModel         = 0x0110
	tagDateTime      = 0x0132
	tagExifIFD       = 0x8769
	tagExposureTime  = 0x829A
	tagFNumber       = 0x829D
	tagISO           = 0x8827
	tagFocalLength   = 0x920A
	tagLensModel     = 0xA434
	typeByte         = 1
	typeASCII        = 2
	typeShort        = 3
	typeLong         = 4
	typeRational     = 5
	typeUndefined    = 7
	typeSLONG        = 9
	typeSRational    = 10
	jpegSOI          = 0xD8
	jpegAPP1         = 0xE1
	jpegSOS          = 0xDA
	tiffIdentifier   = 0x002A
	exifHeaderLength = 6
)

type Metadata struct {
	Make         string `json:"make"`
	Model        string `json:"model"`
	DateTime     string `json:"date_time"`
	LensModel    string `json:"lens_model"`
	ExposureTime string `json:"exposure_time"`
	FNumber      string `json:"f_number"`
	ISO          string `json:"iso"`
	FocalLength  string `json:"focal_length"`
}

func Extract(path string, source ImageSource) (Metadata, error) {
	reader, err := source.Open(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("open image: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return Metadata{}, fmt.Errorf("read image: %w", err)
	}

	metadata, err := parseExif(data)
	if err != nil {
		return Metadata{}, err
	}

	return metadata, nil
}

type tiffData struct {
	order binary.ByteOrder
	data  []byte
}

type tagValue struct {
	tagType uint16
	value   string
}

func parseExif(data []byte) (Metadata, error) {
	exifBlock, err := findExifBlock(data)
	if err != nil {
		return Metadata{}, err
	}

	if len(exifBlock) < exifHeaderLength+8 {
		return Metadata{}, errors.New("exif block too short")
	}

	tiffStart := exifHeaderLength
	tiffBytes := exifBlock[tiffStart:]

	order, err := parseByteOrder(tiffBytes)
	if err != nil {
		return Metadata{}, err
	}

	if order.Uint16(tiffBytes[2:4]) != tiffIdentifier {
		return Metadata{}, errors.New("invalid tiff header")
	}

	tiff := tiffData{
		order: order,
		data:  tiffBytes,
	}

	ifdOffset := order.Uint32(tiffBytes[4:8])
	tags, err := tiff.parseIFD(ifdOffset)
	if err != nil {
		return Metadata{}, err
	}

	if pointer, ok := tags[tagExifIFD]; ok {
		exifOffset, err := parseUint32(pointer.value)
		if err == nil {
			ifdTags, err := tiff.parseIFD(exifOffset)
			if err == nil {
				for tag, value := range ifdTags {
					tags[tag] = value
				}
			}
		}
	}

	metadata := Metadata{
		Make:         tags[tagMake].value,
		Model:        tags[tagModel].value,
		LensModel:    tags[tagLensModel].value,
		ExposureTime: tags[tagExposureTime].value,
		FNumber:      tags[tagFNumber].value,
		ISO:          tags[tagISO].value,
		FocalLength:  tags[tagFocalLength].value,
	}

	if rawDate := tags[tagDateTime].value; rawDate != "" {
		if parsed, err := time.Parse("2006:01:02 15:04:05", rawDate); err == nil {
			metadata.DateTime = parsed.Format(time.RFC3339)
		} else {
			metadata.DateTime = rawDate
		}
	}

	return metadata, nil
}

func findExifBlock(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != jpegSOI {
		return nil, errors.New("not a jpeg image")
	}

	offset := 2
	for offset+4 <= len(data) {
		if data[offset] != 0xFF {
			return nil, errors.New("invalid jpeg marker")
		}

		marker := data[offset+1]
		if marker == jpegSOS {
			break
		}

		segmentLength := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if segmentLength < 2 || offset+2+segmentLength > len(data) {
			return nil, errors.New("invalid jpeg segment length")
		}

		segmentStart := offset + 4
		segmentEnd := offset + 2 + segmentLength

		if marker == jpegAPP1 && segmentEnd-segmentStart >= exifHeaderLength {
			segment := data[segmentStart:segmentEnd]
			if string(segment[:exifHeaderLength]) == "Exif\x00\x00" {
				return segment, nil
			}
		}

		offset = segmentEnd
	}

	return nil, errors.New("exif data not found")
}

func parseByteOrder(data []byte) (binary.ByteOrder, error) {
	if len(data) < 2 {
		return nil, errors.New("tiff header too short")
	}

	switch string(data[:2]) {
	case "II":
		return binary.LittleEndian, nil
	case "MM":
		return binary.BigEndian, nil
	default:
		return nil, errors.New("unknown byte order")
	}
}

func (t tiffData) parseIFD(offset uint32) (map[uint16]tagValue, error) {
	if offset == 0 {
		return map[uint16]tagValue{}, nil
	}

	if int(offset)+2 > len(t.data) {
		return nil, errors.New("ifd offset out of range")
	}

	entryCount := t.order.Uint16(t.data[offset : offset+2])
	entriesStart := int(offset + 2)
	entriesEnd := entriesStart + int(entryCount)*12
	if entriesEnd > len(t.data) {
		return nil, errors.New("ifd entries out of range")
	}

	values := make(map[uint16]tagValue)
	for i := 0; i < int(entryCount); i++ {
		entryOffset := entriesStart + i*12
		tag := t.order.Uint16(t.data[entryOffset : entryOffset+2])
		tagType := t.order.Uint16(t.data[entryOffset+2 : entryOffset+4])
		count := t.order.Uint32(t.data[entryOffset+4 : entryOffset+8])
		valueOffset := t.order.Uint32(t.data[entryOffset+8 : entryOffset+12])

		value, ok := t.readValue(tagType, count, valueOffset)
		if ok {
			values[tag] = tagValue{tagType: tagType, value: value}
		}
	}

	return values, nil
}

func (t tiffData) readValue(tagType uint16, count uint32, valueOffset uint32) (string, bool) {
	switch tagType {
	case typeASCII:
		return t.readASCII(count, valueOffset)
	case typeRational:
		return t.readRational(count, valueOffset, false)
	case typeSRational:
		return t.readRational(count, valueOffset, true)
	case typeShort:
		return t.readShort(count, valueOffset)
	case typeLong:
		return t.readLong(count, valueOffset)
	case typeByte, typeUndefined:
		return t.readByte(count, valueOffset)
	case typeSLONG:
		return t.readSignedLong(count, valueOffset)
	default:
		return "", false
	}
}

func (t tiffData) readASCII(count uint32, valueOffset uint32) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count, valueOffset)
	if data == nil {
		return "", false
	}

	for i, b := range data {
		if b == 0x00 {
			return string(data[:i]), true
		}
	}

	return string(data), true
}

func (t tiffData) readRational(count uint32, valueOffset uint32, signed bool) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count*8, valueOffset)
	if data == nil {
		return "", false
	}

	if signed {
		numerator := int32(t.order.Uint32(data[0:4]))
		denominator := int32(t.order.Uint32(data[4:8]))
		if denominator == 0 {
			return "", false
		}
		return fmt.Sprintf("%d/%d", numerator, denominator), true
	}

	numerator := t.order.Uint32(data[0:4])
	denominator := t.order.Uint32(data[4:8])
	if denominator == 0 {
		return "", false
	}
	return fmt.Sprintf("%d/%d", numerator, denominator), true
}

func (t tiffData) readShort(count uint32, valueOffset uint32) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count*2, valueOffset)
	if data == nil {
		return "", false
	}

	value := t.order.Uint16(data[:2])
	return fmt.Sprintf("%d", value), true
}

func (t tiffData) readLong(count uint32, valueOffset uint32) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count*4, valueOffset)
	if data == nil {
		return "", false
	}

	value := t.order.Uint32(data[:4])
	return fmt.Sprintf("%d", value), true
}

func (t tiffData) readSignedLong(count uint32, valueOffset uint32) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count*4, valueOffset)
	if data == nil {
		return "", false
	}

	value := int32(t.order.Uint32(data[:4]))
	return fmt.Sprintf("%d", value), true
}

func (t tiffData) readByte(count uint32, valueOffset uint32) (string, bool) {
	if count == 0 {
		return "", false
	}

	data := t.readData(count, valueOffset)
	if data == nil {
		return "", false
	}

	return fmt.Sprintf("%x", data[0]), true
}

func (t tiffData) readData(size uint32, valueOffset uint32) []byte {
	if size <= 4 {
		buffer := make([]byte, 4)
		t.order.PutUint32(buffer, valueOffset)
		return buffer[:size]
	}

	start := int(valueOffset)
	end := start + int(size)
	if start < 0 || end > len(t.data) {
		return nil
	}

	return t.data[start:end]
}

func parseUint32(value string) (uint32, error) {
	var parsed uint32
	_, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
