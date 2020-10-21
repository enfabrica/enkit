package karchive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Untarz(name string, r io.Reader, dest string) error {
	_, d, err := Decoder(name, r)
	if err != nil {
		return err
	}
	return Untar(d, dest)
}

func Untar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		f, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		abs := filepath.Join(dir, filepath.FromSlash(f.Name))
		mode := f.FileInfo().Mode()

		switch f.Typeflag {
		case tar.TypeSymlink:
			dest := filepath.Join(dir, filepath.FromSlash(f.Linkname))
			if err := os.Symlink(dest, abs); err != nil {
				return fmt.Errorf("could not create link %s to %s: %w", abs, dest, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				return err
			}

			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
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

			if f.ModTime.IsZero() {
				break
			}
			if err := os.Chtimes(abs, f.AccessTime, f.ModTime); err != nil {
				return fmt.Errorf("could not set time of file %s: %w", abs, err)
			}
		case tar.TypeDir:
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			if f.ModTime.IsZero() {
				break
			}
			if err := os.Chtimes(abs, f.AccessTime, f.ModTime); err != nil {
				return fmt.Errorf("could not set time of file %s: %w", abs, err)
			}

		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}
