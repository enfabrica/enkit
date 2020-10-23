package karchive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type options struct {
	// File umask, and dir umask.
	fumask, dumask uint32

	// Default directory file mode.
	dirmode os.FileMode
}

type Modifier func(*options)

type Modifiers []Modifier

func (m Modifiers) Apply(o *options) {
	for _, mod := range m {
		mod(o)
	}
}

// WithFileUmask sets an umask for written files.
//
// The default file umask is 0, meaning that whatever is set in the .tar file
// will actually be used.
//
// For example: WithFileUmask(0222) will result in no file being writable.
func WithFileUmask(umask uint32) Modifier {
	return func(o *options) {
		o.fumask = umask
	}
}

// WithDirUmask sets an umask for written directories.
//
// The default dir umask is 0, meaning that whatever is set in the .tar file
// will actually be used.
//
// For example: WithDirUmask(0222) will result in no dir being writable.
func WithDirUmask(umask uint32) Modifier {
	return func(o *options) {
		o.dumask = umask
	}
}

// WithDefaultDirMode sets the privileges to use to create unknown directories.
//
// Tar files normally contain directory and file definitions, with files in sub
// directories always appearing after the definition of the directory they appear in.
//
// However, this is not mandated. There can be tar files that define files, but
// not the directories. Or where the directory definition is after the file definition.
//
// WithDefaultDirMode defines the mode to use to create directories that are necessary
// to unpack a file, but for which a definition has not been seen in the tar yet.
//
// If a definition is seen later on while unpacking the file, that definition will
// be used, and the privileges here will only be temporarily used.
// If a definition is not seen, the privileges here will be the final ones.
func WithDefaultDirMode(mode os.FileMode) Modifier {
	return func(o *options) {
		o.dirmode = mode
	}
}

// Untarz opens a .tar.{gz,xz,bz2} file, and unpacks it by invoking Untar.
func Untarz(name string, r io.Reader, dest string, mods ...Modifier) error {
	_, d, err := Decoder(name, r)
	if err != nil {
		return err
	}
	return Untar(d, dest, mods...)
}

// Untar opens a .tar file (no compression), and unpacks it in the specified directory.
//
// Untar can only create regular files, symlinks, and directories.
// The presence of any other kind of file in the archive will cause the opening to fail.
//
// Except for targets of symlinks, if the tar contains files named like '../../../', they
// won't be allowed to escape the unpack directory: all unpacked files will be placed in
// a subdirectory of dir, no matter what.
func Untar(r io.Reader, dir string, mods ...Modifier) error {
	o := options{dirmode: 0755}
	Modifiers(mods).Apply(&o)

	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("could not compute absolute path of %s - %w", dir, err)
	}

	type delayed struct {
		path        string
		mode        os.FileMode
		access, mod time.Time
	}

	dirs := map[string]*delayed{}
	tr := tar.NewReader(r)
	for {
		f, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Without the extra '/' and filepath.Clean, a Name like ../../../etc could
		// result in overwriting arbitrary files on the system.
		abs := filepath.Join(dir, filepath.Clean("/"+filepath.FromSlash(f.Name)))
		mode := f.FileInfo().Mode()

		switch f.Typeflag {
		case tar.TypeSymlink:
			dest := filepath.Join(dir, filepath.FromSlash(f.Linkname))
			if err := os.Symlink(dest, abs); err != nil {
				return fmt.Errorf("could not create link %s to %s: %w", abs, dest, err)
			}
		case tar.TypeReg:
			// The MkdirAll here is to tolerate .tar.gz files that don't include directory creation entries.
			created, err := MkdirAll(filepath.Dir(abs), 0700)
			if err != nil {
				return err
			}
			for _, dir := range created {
				dirs[dir] = &delayed{path: dir, mode: o.dirmode}
			}

			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}

			n, err := io.Copy(wf, tr)
			if err != nil {
				return fmt.Errorf("could not write %s: %w", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("could not write %s: tar indicates %d bytes, only %d written", abs, f.Size, n)
			}
			if err := wf.Close(); err != nil {
				return fmt.Errorf("closing %s: %w", abs, err)
			}

			if !f.ModTime.IsZero() || !f.AccessTime.IsZero() {
				if err := os.Chtimes(abs, f.AccessTime, f.ModTime); err != nil {
					return fmt.Errorf("could not set time of file %s: %w", abs, err)
				}
			}

			if err := os.Chmod(abs, os.FileMode(uint32(mode.Perm()) & ^o.fumask)); err != nil {
				return fmt.Errorf("could not chmod directory %s: %w", abs, err)
			}

		case tar.TypeDir:
			created, err := MkdirAll(abs, 0700)
			if err != nil {
				return err
			}
			for _, dir := range created {
				dirs[dir] = &delayed{path: dir, mode: o.dirmode}
			}
			dirs[abs] = &delayed{path: abs, mode: mode.Perm(), access: f.AccessTime, mod: f.ModTime}

		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}

	sorted := []*delayed{}
	for _, v := range dirs {
		sorted = append(sorted, v)
	}

	// The requested mask / privileges may cause a dir to become not writable.
	// To apply the privileges correctly, we need to move from the innermost directory to the
	// outermost one.
	//
	// Doing this "properly" would require building a tree. But we're slackers.
	// What we do instead is just fix the privileges in reverse alphabetical order.
	// Guess what? In reverse alphabetical order, a subdirectory is guaranteed to appear
	// before its parent directory.
	//
	// Well, modulo weird internationalization rules, which I believe do not apply to a simple >.
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].path > sorted[j].path
	})

	for _, dir := range dirs {
		if !dir.mod.IsZero() || !dir.access.IsZero() {
			if err := os.Chtimes(dir.path, dir.access, dir.mod); err != nil {
				return fmt.Errorf("could not set time of file %s: %w", dir.path, err)
			}
		}
		if err := os.Chmod(dir.path, os.FileMode(uint32(dir.mode.Perm()) & ^o.dumask)); err != nil {
			return fmt.Errorf("could not chmod: %w", err)
		}
	}

	return nil
}
