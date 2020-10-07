package kapt

import (
	"github.com/cybozu-go/aptutil/apt"
	"github.com/enfabrica/enkit/kbuild/common"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type Origin struct {
	Watch *common.Watch

	Distribution string
	Component    string
	Arch         string

	Repo *Repository
}

type Paragraph struct {
	apt.Paragraph
	Origin Origin
}

func (p Paragraph) Single(key string) string {
	values := p.Paragraph[key]
	if len(values) <= 0 {
		return ""
	}
	return values[0]
}

func (p Paragraph) URLFile() *url.URL {
	path := p.Single("Filename")
	if p.Origin.Repo == nil {
		return nil
	}

	return p.Origin.Repo.URLDeb(path)
}

var kpackageVersion = regexp.MustCompile(`^((?U:.*))-([0-9.]+(?:-rc[0-9]+)?)(?:-([0-9]+))?(?:-(\S+))?$`)

func (p Paragraph) KVersion() *common.KVersion {
	arch := p.Single("Architecture")
	full := p.Single("Package")

	name := strings.TrimSuffix(full, "-"+arch)
	match := kpackageVersion.FindStringSubmatch(name)
	if len(match) != 5 {
		return nil
	}

	return &common.KVersion{
		Full:    full,
		Name:    name,
		Package: p.Single("Version"),
		Arch:    arch,
		Type:    match[1],
		Kernel:  match[2],
		Upload:  match[3],
		Variant: match[4],
	}

}

type Alternative struct {
	Name, Constraints string
}

type Dependency struct {
	Alternative []Alternative
}

func (d Dependency) First() Alternative {
	if len(d.Alternative) >= 1 {
		return d.Alternative[0]
	}
	return Alternative{Name: "<not-available>"}
}

func (p Paragraph) Package() string {
	return path.Join(p.Origin.Watch.Output, p.Single("Package"))
}

func (p Paragraph) Get(key string) ([]string, bool) {
	v, f := p.Paragraph[key]
	return v, f
}

var splitDeps = regexp.MustCompile(`\s*,\s*`)
var splitAlternatives = regexp.MustCompile(`\s*\|\s*`)
var splitConstraints = regexp.MustCompile(`\s+`)

func (p Paragraph) Dependencies() []Dependency {
	result := []Dependency{}
	deps := splitDeps.Split(p.Single("Depends"), -1)
	for _, dep := range deps {
		alts := splitAlternatives.Split(dep, -1)
		depr := Dependency{}
		for _, alt := range alts {
			nc := splitConstraints.Split(alt, 2)
			if len(nc) <= 0 {
				continue
			}
			altr := Alternative{
				Name: nc[0],
			}
			if len(nc) > 1 {
				altr.Constraints = nc[1]
			}
			depr.Alternative = append(depr.Alternative, altr)
		}
		result = append(result, depr)
	}
	return result
}
