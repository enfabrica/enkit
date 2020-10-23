package karchive

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// MkdirAll is just like os.MkdirAll, except it returns the set of directories created.
func MkdirAll(dir string, perm os.FileMode) ([]string, error) {
	dir = filepath.Clean(dir)

	chunk := dir
	tocreate := []string{}
	for {
		stat, err := os.Stat(chunk)
		if err == nil {
			if stat.IsDir() {
				break
			}
			return tocreate, &os.PathError{Op: "mkdir", Path: chunk, Err: syscall.ENOTDIR}
		}
		tocreate = append(tocreate, chunk)

		ix := strings.LastIndex(chunk, string(os.PathSeparator))
		if ix <= 0 {
			break
		}
		chunk = chunk[:ix]
	}

	for ix := len(tocreate) - 1; ix >= 0; ix-- {
		err := os.Mkdir(tocreate[ix], perm)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return tocreate, err
		}
	}

	return tocreate, nil
}
