package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/cybozu-go/aptutil/apt"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/scheduler"
	"github.com/enfabrica/enkit/lib/khttp/workpool"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/kbuild/assets"
	"github.com/ulikunitz/xz"
	"github.com/xor-gate/ar"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
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

type Origin struct {
	Watch *Watch

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

// Decoder returns an io.Reader capable of decoding the file.
func (cu ControlURL) Decoder(current io.Reader) (string, io.Reader, error) {
	return Decoder(cu.Path, current)
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

type Section struct {
	Size   uint64
	SHA256 string
	Path   string
}

type SectionMap map[string]Section

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

type Watch struct {
	Output string

	Mirror       string
	Distribution []string
	Component    []string
	Arch         []string
}

// Represents the version of a kernel package.
//
// For a package named:
//   linux-headers-5.8.0-rc9-2-cloud-amd64
//
// We would have:
//   Name: linux-headers-5.8.0-2-cloud
//   Package: <a package version - independent of the name>
//   Arch: amd64
//   Type: linux-headers
//   Kernel: 5.8.0-rc9
//   Upload: 2
//   Variant: cloud
type KVersion struct {
	Full string

	Name    string
	Package string
	Type    string

	Kernel  string
	Upload  string
	Variant string
	Arch    string
}

func (kv KVersion) Id() string {
	id := kv.ArchLessId()
	if kv.Arch != "" {
		id += "-" + kv.Arch
	}
	return id
}

func (kv KVersion) ArchLessId() string {
	id := kv.Kernel
	if kv.Upload != "" {
		id += "-" + kv.Upload
	}
	if kv.Variant != "" {
		id += "-" + kv.Variant
	}
	return id
}

var kpackageVersion = regexp.MustCompile(`^((?U:.*))-([0-9.]+(?:-rc[0-9]+)?)(?:-([0-9]+))?(?:-(\S+))?$`)

func ParseKVersion(p Paragraph) *KVersion {
	arch := p.Single("Architecture")
	full := p.Single("Package")

	name := strings.TrimSuffix(full, "-"+arch)
	match := kpackageVersion.FindStringSubmatch(name)
	if len(match) != 5 {
		return nil
	}

	return &KVersion{
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

type Fetch struct {
	Package Paragraph
	Groups  []*Group
}

type Group struct {
	Version KVersion
	Package []Paragraph

	Lock   sync.Mutex
	Writer *tar.Writer
	Fixups Fixups
}

func writeArchiveFileEntry(archive, file string, hdr *tar.Header, t io.Reader, groups []*Group) error {
	writers := []io.Writer{}
	for _, g := range groups {
		g.Lock.Lock()
		defer g.Lock.Unlock()

		err := g.Writer.WriteHeader(hdr)
		if err != nil {
			return fmt.Errorf("tar write error for %s - %s, %w", archive, file, err)
		}
		writers = append(writers, g.Writer)

		if (hdr.Typeflag == tar.TypeLink || hdr.Typeflag == tar.TypeSymlink) && filepath.IsAbs(hdr.Linkname) {
			rel, err := filepath.Rel("/", hdr.Linkname)
			if err != nil {
				return fmt.Errorf("could not convert path %s to relative to /", hdr.Linkname)
			}
			g.Fixups.Symlinks = append(g.Fixups.Symlinks, Symlink{
				Path:   filepath.Clean(hdr.Name),
				Target: rel,
			})
		}
	}

	if _, err := io.Copy(io.MultiWriter(writers...), t); err != nil {
		return fmt.Errorf("copy failed %s", err)
	}

	// log.Printf("in %s - %s", archive, hdr.Name)
	return nil

}

func writeArchiveFile(archive, file string, d io.Reader, groups []*Group, mangler func(*tar.Header) error) error {
	t := tar.NewReader(d)
	for {
		hdr, err := t.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("tar decode error for %s - %s, %w", archive, file, err)
		}
		if mangler != nil {
			if err := mangler(hdr); err != nil {
				return err
			}
		}

		if err := writeArchiveFileEntry(archive, file, hdr, t, groups); err != nil {
			return err
		}

	}
	return nil
}

type Symlink struct {
	Path, Target string
}

type Fixups struct {
	Symlinks []Symlink
	Version  KVersion
}

func writeDataFile(archive, file string, d io.Reader, fetch *Fetch) error {
	return writeArchiveFile(archive, file, d, fetch.Groups, nil)
}

func writeControlFile(archive, file string, d io.Reader, fetch *Fetch) error {
	kv := ParseKVersion(fetch.Package)
	mangler := func(hdr *tar.Header) error {
		name := path.Clean(hdr.Name)
		switch name {
		case ".":
			fallthrough
		case "./":
			hdr.Name = "./control"
		default:
			hdr.Name = "./" + filepath.Join("control", kv.Full+"."+name)
		}
		return nil
	}
	return writeArchiveFile(archive, file, d, fetch.Groups, mangler)
}

// ubuntu:
// apt-cache search linux-headers-5.8.0.20
// linux-headers-5.8.0-20 - Header files related to Linux kernel version 5.8.0 - virutal package importing -generic and -lowlatency.
// linux-headers-5.8.0-20-generic - Linux kernel headers for version 5.8.0 on 64 bit x86 SMP
// linux-headers-5.8.0-20-lowlatency - Linux kernel headers for version 5.8.0 on 64 bit x86 SMP
func main() {
	tpl, err := template.New("install").Parse(string(assets.Data["install.sh"]))
	if err != nil || len(assets.Data["install.sh"]) <= 0 {
		log.Fatalf("binary does not include required asset 'install.sh' - %s", err)
	}

	wp, err := workpool.New()
	if err != nil {
		log.Fatalf("workpool failed: %s", err)
	}
	sc := scheduler.New()
	r := retry.New()

	watch := []*Watch{
		{
			Output:    "debian",
			Mirror:    "http://ftp.us.debian.org/debian",
			Component: []string{"main"},
			//Distribution: []string{"unstable"},
			//Arch:         []string{"amd64"},
			Distribution: []string{"stable", "testing", "unstable", "experimental"},
			Arch:         []string{"amd64", "all"},
		},
		{
			Output:       "ubuntu",
			Mirror:       "http://archive.ubuntu.com/ubuntu/",
			Component:    []string{"main"}, // restricted
			Distribution: []string{"groovy"},
			Arch:         []string{"amd64"},
		}}

	to_assemble := regexp.MustCompile(`^linux-headers-|linux-kbuild-|linux-.*-headers`)

	pindex := map[string]Paragraph{}
	for _, w := range watch {
		for _, d := range w.Distribution {
			for _, c := range w.Component {
				for _, a := range w.Arch {
					repo, err := NewRepository(w.Mirror, d)
					if err != nil {
						log.Printf("could not create repository: %v", err)
						continue
					}

					u := repo.URLPackages(c, a).First()
					if u == nil {
						log.Printf("unsupported url for Packages file: %s", u)
						continue
					}

					log.Printf("fetching %s", u.String())
					packs, err := u.Get("Package", to_assemble)
					if err != nil && !errors.Is(err, io.EOF) {
						log.Printf("could not fetch packages file for %s - %s", u.String(), err)
						continue
					}
					for _, p := range packs {
						p.Origin = Origin{
							Watch: w,

							Distribution: d,
							Component:    c,
							Arch:         a,
							Repo:         repo,
						}
						pindex[p.Package()] = p
					}
				}
			}
		}
	}

	// All linux headers packages that depend on the linux-compiler are normally meant to build kernel modules.
	groups := map[string]*Group{}
outer:
	for _, p := range pindex {
		deps := p.Single("Depends")
		// if !strings.Contains(deps, "linux-compiler-") {
		if !strings.Contains(deps, "linux-compiler-") && !(strings.Contains(deps, "libelf") && strings.Contains(deps, "headers")) {
			continue
		}

		version := ParseKVersion(p)
		if version == nil {
			continue
		}
		log.Printf("For %s...", p.Package())
		group := Group{Fixups: Fixups{Version: *version}, Package: []Paragraph{p}}
		for _, dep := range p.Dependencies() {
			name := dep.First().Name
			if !to_assemble.MatchString(name) {
				continue
			}
			name = path.Join(p.Origin.Watch.Output, name)
			log.Printf("  dep %s", name)
			dp, found := pindex[name]
			if !found {
				log.Printf("cannot find dependency %s for %s", name, p.Single("Package"))
				continue outer
			}
			group.Package = append(group.Package, dp)
		}
		groups[path.Join(p.Origin.Watch.Output, version.Id())] = &group
	}

	files := map[string]*Fetch{}
	for name, group := range groups {
		name := name
		group := group

		// FIXME: skip kernel versions we already have!

		if err := os.MkdirAll(path.Dir(name), 0750); err != nil {
			log.Fatalf("error %v", err)
		}

		f, err := os.OpenFile(name+".tar.gz", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			log.Fatalf("error %v", err)
		}
		defer f.Close()

		gz := gzip.NewWriter(f)
		defer gz.Close()

		targz := tar.NewWriter(gz)
		defer targz.Close()

		group.Writer = targz
		defer func() {
			b := strings.Builder{}
			tpl.Execute(&b, group.Fixups)
			script := b.String()

			if err := targz.WriteHeader(&tar.Header{
				Name: fmt.Sprintf("./install-%s.sh", path.Base(name)),
				Mode: 0755,
				Size: int64(len(script)),
			}); err != nil {
				log.Printf("could not save script to file %s", err)
				return
			}
			if l, err := targz.Write([]byte(script)); err != nil || l != len(script) {
				log.Printf("could not save script to directory %s - %d bytes written out of %d", err, l, len(script))
				return
			}
		}()

		for _, p := range group.Package {
			key := p.URLFile().String()
			fetch := files[key]
			if fetch == nil {
				fetch = &Fetch{Package: p}
				files[key] = fetch
			}
			fetch.Groups = append(fetch.Groups, group)
		}
	}

	for file, fetch := range files {
		file := file
		fetch := fetch
		wp.Add(workpool.WithRetry(r, sc, wp, func() error {
			return protocol.Get(file, protocol.Reader(func(hr io.Reader) error {
				r := ar.NewReader(hr)
				for {
					arh, err := r.Next()
					if err != nil {
						if errors.Is(err, io.EOF) {
							return nil
						}
						log.Printf("for file %s: error %s", file, err)
						return err
					}
					log.Printf("for file %s: %v", file, *arh)

					name, d, err := Decoder(arh.Name, r)
					if err != nil {
						return fmt.Errorf("decode error for %s: %w", file, err)
					}
					if !strings.HasSuffix(name, ".tar") {
						data, err := ioutil.ReadAll(d)
						if err != nil {
							return fmt.Errorf("could not read file %s - %s, %w", file, arh.Name, err)
						}
						log.Printf("for %s - %s - got %s", file, arh.Name, data)
						continue
					}
					if strings.HasPrefix(name, "control.") {
						err = writeControlFile(file, arh.Name, d, fetch)
					} else {
						err = writeDataFile(file, arh.Name, d, fetch)
					}
					if err != nil {
						return err
					}
				}
			}))
		}, workpool.ErrorLog(log.Printf)))
		log.Printf("FILE: %s", file)
	}
	wp.Wait()
}
