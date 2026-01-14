package exif

import (
	"errors"
	"fmt"
	"os"
)

var (
	ErrExifNotFound = errors.New("exif data not found")
	ErrInvalidExif  = errors.New("invalid exif data")
)

// FileReader abstracts file access for EXIF extraction.
type FileReader interface {
	ReadFile(name string) ([]byte, error)
}

type osFileReader struct{}

// NewOSFileReader returns a FileReader backed by the local filesystem.
func NewOSFileReader() FileReader {
	return osFileReader{}
}

func (osFileReader) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Summary represents a minimal EXIF summary.
type Summary struct {
	Make             string `json:"make,omitempty"`
	Model            string `json:"model,omitempty"`
	LensModel        string `json:"lens_model,omitempty"`
	DateTime         string `json:"date_time,omitempty"`
	FNumber          string `json:"f_number,omitempty"`
	ExposureTime     string `json:"exposure_time,omitempty"`
	ISOSpeed         string `json:"iso_speed,omitempty"`
	FocalLength      string `json:"focal_length,omitempty"`
	ExposureProgram  string `json:"exposure_program,omitempty"`
	MeteringMode     string `json:"metering_mode,omitempty"`
	WhiteBalance     string `json:"white_balance,omitempty"`
	Software         string `json:"software,omitempty"`
	Orientation      string `json:"orientation,omitempty"`
	Flash            string `json:"flash,omitempty"`
	SceneCaptureType string `json:"scene_capture_type,omitempty"`
}

// ExtractSummary reads the file at path and extracts an EXIF summary.
func ExtractSummary(reader FileReader, path string) (Summary, error) {
	data, err := reader.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read file: %w", err)
	}

	summary, err := parseEXIF(data)
	if err != nil {
		return Summary{}, err
	}

	return summary, nil
}
