package common

import (
	"compress/gzip"
	"fmt"
	"github.com/ulikunitz/xz"
	"io"
	"path"
	"strings"
)

func Decoder(name string, current io.Reader) (string, io.Reader, error) {
	ext := path.Ext(name)
	name = strings.TrimSuffix(name, ext)
	switch ext {
	case "": // Plain text, no extension
		return name, current, nil
	case ".xz":
		r, err := xz.NewReader(current)
		return name, r, err
	case ".gz":
		r, err := gzip.NewReader(current)
		return name, r, err
	}
	return "", nil, fmt.Errorf("format of file not known - extension %s does not match any known format", ext)
}
