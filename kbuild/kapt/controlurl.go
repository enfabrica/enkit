package kapt

import (
	"github.com/cybozu-go/aptutil/apt"
	"github.com/enfabrica/enkit/kbuild/common"
	"github.com/enfabrica/enkit/lib/khttp/protocol"

	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
)

// ControlURL represents an URL pointing to a file in the debian control file format.
// Debian repositories use this format for Release files, as well as Packages and
// the proper control files, described here:
//     https://www.debian.org/doc/debian-policy/ch-controlfields.html#syntax-of-control-files
//
// Control files can generally be parsed with the cybozu-go/aptutil/apt library in go.
type ControlURL struct {
	url.URL
}

// Decoder returns an io.Reader capable of decoding the file.
func (cu ControlURL) Decoder(current io.Reader) (string, io.Reader, error) {
	return common.Decoder(cu.Path, current)
}

// Supported returns true if the URL specifies a format that can be opened by this library.
func (cu ControlURL) Supported() bool {
	ext := path.Ext(cu.Path)
	switch ext {
	case "": // Plain text has no extension.
		return true
	case ".xz":
		return true
	case ".gz":
		return true
	}
	return false
}

// Parse fetches the control file and invoke the handler for each "paragraph" in the file.
// A paragraph is just a section like:
//
//     Package: 7kaa-data
//     Source: 7kaa
//     Version: 2.15.4p1+dfsg-1
//     Installed-Size: 103621
//     [...]
//
func (cu ControlURL) Parse(handler func(Paragraph) error) error {
	return protocol.Get(cu.String(), protocol.Reader(func(input io.Reader) error {
		_, decoder, err := cu.Decoder(input)
		if err != nil {
			return err
		}

		parser := apt.NewParser(decoder)
		for {
			section, err := parser.Read()
			if err != nil {
				return fmt.Errorf("error parsing file: %w", err)
			}

			if err := handler(Paragraph{Paragraph: section}); err != nil {
				return err
			}
		}
	}))
}

// Get will return all the paragraph that have the field matching the specified regular expression.
func (cu ControlURL) Get(field string, re *regexp.Regexp) ([]Paragraph, error) {
	result := []Paragraph{}
	return result, cu.Parse(func(p Paragraph) error {
		value, found := p.Get(field)
		if !found {
			return nil
		}
		for _, v := range value {
			if re.MatchString(v) {
				result = append(result, p)
				return nil
			}
		}
		return nil
	})
}

type ControlURLs []ControlURL

func (cu ControlURLs) First() *ControlURL {
	for _, u := range cu {
		if u.Supported() {
			return &u
		}
	}
	return nil
}

func (cu ControlURLs) Parse(handler func(Paragraph) error) error {
	u := cu.First()
	if u == nil {
		return fmt.Errorf("None of the URLs is supported %v", cu)
	}
	return u.Parse(handler)
}
