// Config loaders to read/write files in directories.
package directory

import (
	"github.com/kirsle/configdir"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

type Directory struct {
	path string
}

// Returns the absolute path to a specific folder within the
// system default configuration directory for the current user.
//
// On Linux systems, this generally means ~/.config/<app>/<namespace>
func GetConfigDir(app string, namespaces ...string) (string, error) {
	paths := append([]string{app}, namespaces...)
	dir := configdir.LocalConfig(paths...)
	if !filepath.IsAbs(dir) {
		user, err := user.Current()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(user.HomeDir, dir)
	}

	return dir, nil
}

// OpenHomeDir returns a Loader capable of loading and creating config
// files in the system default configuration directory for the current
// user.
//
// On Linux systems, this generally means ~/.config/<app>/<namespace>/.
func OpenHomeDir(app string, namespaces ...string) (*Directory, error) {
	dir, err := GetConfigDir(app, namespaces...)
	if err != nil {
		return nil, err
	}

	return &Directory{path: dir}, nil
}

// Refresh values cached by OpenHomeDir.
//
// Internally, OpenHomeDir caches some of the computed paths. Refresh() will cause
// those paths to be re-computed.
//
// Don't bother calling Refresh() unless your project mingles with the HOME
// environment variable, or variables like XDG_CONFIG_HOME.
func Refresh() {
	configdir.Refresh()
}

// OpenDir returns a Loader capable of loading and creating config
// files in the specified directory.
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

func (hd *Directory) Delete(name string) error {
	path := filepath.Join(hd.path, name)
	return os.Remove(path)
}

func (hd *Directory) Read(name string) ([]byte, error) {
	path := filepath.Join(hd.path, name)
	return ioutil.ReadFile(path)
}

func (hd *Directory) Write(name string, data []byte) error {
	if err := os.MkdirAll(hd.path, 0770); err != nil {
		return err
	}

	// Don't write the file in place, use rename to guarantee filesystem atomicity.
	tmp, err := ioutil.TempFile(hd.path, name)
	if err != nil {
		return err
	}
	tmp.Close()

	if err := ioutil.WriteFile(tmp.Name(), data, 0660); err != nil {
		os.Remove(tmp.Name())
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(hd.path, name))
}
