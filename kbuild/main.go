package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/kbuild/common"
	"github.com/enfabrica/enkit/kbuild/kapt"

	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/scheduler"
	"github.com/enfabrica/enkit/lib/khttp/workpool"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/kbuild/assets"
	"github.com/xor-gate/ar"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
)

type Fetch struct {
	Package kapt.Paragraph
	Groups  []*Group
}

type Group struct {
	Version common.KVersion
	Package []kapt.Paragraph

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
	Version  common.KVersion
}

func writeDataFile(archive, file string, d io.Reader, fetch *Fetch) error {
	return writeArchiveFile(archive, file, d, fetch.Groups, nil)
}

func writeControlFile(archive, file string, d io.Reader, fetch *Fetch) error {
	kv := fetch.Package.KVersion()
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

type Config struct {
	// List of repositories to watch.
	Watch []*common.Watch

	// Regular expression selecting which packages contain kernel headers.
	Packages string
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

	var config Config
	if err := marshal.UnmarshalAsset("watch", assets.Data, &config); err != nil {
		log.Fatalf("failed parsing config: %s", err)
	}
	to_assemble, err := regexp.Compile(config.Packages)
	if err != nil {
		log.Fatalf("config had an invalid regexp %s: %s", config.Packages, err)
	}
	log.Printf("Parsed config: %#v", config)

	watch := config.Watch
	pindex := map[string]kapt.Paragraph{}
	for _, w := range watch {
		for _, d := range w.Distribution {
			for _, c := range w.Component {
				for _, a := range w.Arch {
					repo, err := kapt.NewRepository(w.Mirror, d)
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
						p.Origin = kapt.Origin{
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

		version := p.KVersion()
		if version == nil {
			continue
		}
		log.Printf("For %s...", p.Package())
		group := Group{Fixups: Fixups{Version: *version}, Package: []kapt.Paragraph{p}}
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

					name, d, err := common.Decoder(arh.Name, r)
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
