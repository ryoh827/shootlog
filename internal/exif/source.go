package exif

import (
	"io"
	"os"
)

type OSImageSource struct{}

func (OSImageSource) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
