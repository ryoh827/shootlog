package exif

import (
	"errors"
	"testing"
)

func TestExtractSummary(t *testing.T) {
	exifJPEG := buildExifJPEG(t, "TestMake")
	noExifJPEG := []byte{0xFF, 0xD8, 0xFF, 0xD9}

	cases := []struct {
		name       string
		data       []byte
		wantMake   string
		expectErr  error
		expectLens bool
	}{
		{
			name:     "basic exif",
			data:     exifJPEG,
			wantMake: "TestMake",
		},
		{
			name:      "no exif",
			data:      noExifJPEG,
			expectErr: ErrExifNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := stubReader{data: tc.data}
			summary, err := ExtractSummary(reader, "fixture.jpg")
			if tc.expectErr != nil {
				if !errors.Is(err, tc.expectErr) {
					t.Fatalf("expected error %v, got %v", tc.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if summary.Make != tc.wantMake {
				t.Fatalf("expected make %q, got %q", tc.wantMake, summary.Make)
			}
		})
	}
}

type stubReader struct {
	data []byte
}

func (s stubReader) ReadFile(string) ([]byte, error) {
	return s.data, nil
}

func buildExifJPEG(t *testing.T, makeTag string) []byte {
	t.Helper()

	makeBytes := append([]byte(makeTag), 0x00)

	header := append([]byte{'I', 'I'}, 42, 0)
	header = append(header, 8, 0, 0, 0)

	entryCount := []byte{1, 0}
	entry := []byte{0x0F, 0x01, 0x02, 0x00}
	entry = append(entry, byte(len(makeBytes)), 0, 0, 0)
	entry = append(entry, 26, 0, 0, 0)
	nextIFD := []byte{0, 0, 0, 0}

	tiff := append(header, entryCount...)
	tiff = append(tiff, entry...)
	tiff = append(tiff, nextIFD...)
	tiff = append(tiff, makeBytes...)

	exif := append([]byte("Exif\x00\x00"), tiff...)
	length := len(exif) + 2
	if length > 0xFFFF {
		t.Fatalf("exif payload too large: %d", length)
	}
	app1 := []byte{0xFF, 0xE1, byte(length >> 8), byte(length)}
	app1 = append(app1, exif...)

	jpeg := []byte{0xFF, 0xD8}
	jpeg = append(jpeg, app1...)
	jpeg = append(jpeg, 0xFF, 0xD9)

	return jpeg
}
