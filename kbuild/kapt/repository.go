package kapt

import (
	"log"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Repository struct {
	Mirror       *url.URL
	Distribution string
}

func (r *Repository) URLBase() *url.URL {
	result := *r.Mirror
	result.Path = path.Join(result.Path, "/dists/", r.Distribution)
	return &result
}

func (r *Repository) URLRelease() *ControlURL {
	result := ControlURL{URL: *r.URLBase()}
	result.Path = path.Join(result.Path, "Release")
	return &result
}

func (r *Repository) URLDeb(deb string) *url.URL {
	result := *r.Mirror
	result.Path = path.Join(result.Path, deb)
	return &result
}

type Section struct {
	Size   uint64
	SHA256 string
	Path   string
}

type SectionMap map[string]Section

// URLPackages returns a set of URLs where the "Packages" file can be found.
// The Packages file is a file listing all the .deb packages available for the distribution.
//
// arch is an architecture, like "all", "amd64", ...
// component is a string like "main", "non-free", "contrib", ...
func (r *Repository) URLPackages(component, arch string) ControlURLs {
	result := []ControlURL{}
	for _, pf := range []string{"Packages.xz", "Packages.gz", "Packages.diff/Index", "Packages"} {
		url := *r.URLBase()
		pf = path.Join(component, "binary-"+arch, pf)
		url.Path = path.Join(url.Path, pf)
		result = append(result, ControlURL{URL: url})
	}
	return result
}

func (r *Repository) Section() (SectionMap, error) {
	result := SectionMap{}
	return result, r.URLRelease().Parse(
		func(section Paragraph) error {
			files, ok := section.Get("SHA256")
			if !ok {
				log.Printf("File does not have 'SHA256' section")
				return nil
			}

			for _, file := range files {
				fields := strings.Fields(file)
				if len(fields) < 2 {
					log.Printf("skipping line with not enough fields %s", file)
					continue
				}

				size, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					log.Printf("skipping unparsable line %s", err)
					continue
				}
				result[fields[2]] = Section{
					Path:   fields[2],
					SHA256: fields[0],
					Size:   size,
				}
				log.Printf("got file %s", file)
			}
			return nil
		})
}

func NewRepository(mirror, distribution string) (*Repository, error) {
	u, err := url.Parse(mirror)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Mirror:       u,
		Distribution: distribution,
	}, nil
}
