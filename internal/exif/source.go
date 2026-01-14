package exif

import (
	"io"
	"os"
)

type ImageSource interface {
	Open(path string) (io.ReadCloser, error)
}

type OSImageSource struct{}

func (OSImageSource) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
