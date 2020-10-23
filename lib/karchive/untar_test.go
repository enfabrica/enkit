package karchive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrazyUntar(t *testing.T) {
	buf := bytes.Buffer{}

	tw := tar.NewWriter(&buf)

	var files = []struct {
		Path, Body string
	}{
		{"../../../gramsci.wisdom", "If you beat your head against the wall, it is your head that breaks and not the wall."},
		{"test/toat/../../../mandela.wisdom", "It always seems impossible until it's done."},
		{"well/a/simple/file/do-not-resent.txt", "Resentment is like drinking poison and then hoping it will kill your enemies."},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Path,
			Mode: 0440,
			Size: int64(len(file.Body)),
		}
		err := tw.WriteHeader(hdr)
		assert.Nil(t, err)
		_, err = tw.Write([]byte(file.Body))
		assert.Nil(t, err)
	}

	for _, dir := range []string{"./well/a/simple/", "../../../escape/", "good/directory/"} {
		hdr := &tar.Header{
			Name: dir,
			Mode: 0540,
		}
		tw.WriteHeader(hdr)
	}
	err := tw.Close()
	assert.Nil(t, err)

	td, err := ioutil.TempDir("", "untar-test")
	assert.Nil(t, err)

	t.Logf("unpacking in %s", td)
	err = Untar(&buf, td)
	assert.Nil(t, err, "%v", err)

	found := []string{}
	filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
		if path == td {
			return nil
		}

		size := int64(0)
		if !info.Mode().IsDir() {
			size = info.Size()
		}
		found = append(found, fmt.Sprintf("%s %o %d", strings.TrimPrefix(path, td), info.Mode().Perm(), size))
		return nil
	})

	expected := []string{
		"/escape 540 0",
		"/good 755 0",
		"/good/directory 540 0",
		"/gramsci.wisdom 440 85",
		"/mandela.wisdom 440 43",
		"/well 755 0",
		"/well/a 755 0",
		"/well/a/simple 540 0",
		"/well/a/simple/file 755 0",
		"/well/a/simple/file/do-not-resent.txt 440 77",
	}
	assert.Equal(t, expected, found)
}
