package directory

import (
	"github.com/kirsle/configdir"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type Directory struct {
	path string
}

// OpenHomeDir returns a Loader() capable of loading and creating config
// files in the default location for user configs.
//
// On Linux systems, this generally means ~/.config/<app>/<namespace>/.
func OpenHomeDir(app string, namespaces ...string) (*Directory, error) {
	paths := append([]string{app}, namespaces...)
	dir := configdir.LocalConfig(paths...)
	log.Printf("DIR %s", dir)
	if !filepath.IsAbs(dir) {
		user, err := user.Current()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(user.HomeDir, dir)
	}

	return &Directory{path: dir}, nil
}

// Refresh values cached by OpenHomeDir.
//
// Internally, OpenHomeDir caches some of the computed paths. Refresh() will cause
// those paths to be re-computed. This is needed pretty much only if you expect
// environment variables to change value in between calls.
func Refresh() {
	configdir.Refresh()
}

func OpenDir(base string, sub ...string) (*Directory, error) {
	path := filepath.Join(append([]string{base}, sub...)...)
	return &Directory{path: path}, nil
}

func (hd *Directory) List() ([]string, error) {
	files, err := ioutil.ReadDir(hd.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := []string{}
	for _, file := range files {
		if !file.Mode().IsRegular() {
			continue
		}
		paths = append(paths, file.Name())
	}
	return paths, nil
}

func (hd *Directory) Read(name string) ([]byte, error) {
	path := filepath.Join(hd.path, name)
	return ioutil.ReadFile(path)
}

func (hd *Directory) Write(name string, data []byte) error {
	if err := os.MkdirAll(hd.path, 0770); err != nil {
		return err
	}

	path := filepath.Join(hd.path, name)
	return ioutil.WriteFile(path, data, 0660)
}
