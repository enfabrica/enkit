package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Source:  https://gosamples.dev/unzip-file/  (slightly modified)

// Unzip the contents of the source file to the destination location.
// Note that any subdirectories contained within the zip file are
// created within the destination location as a subtree.
func unzipSource(source, destination string) ([]string, error) {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Get the absolute path of the destination.
	destination, err = filepath.Abs(destination)
	if err != nil {
		return nil, err
	}

	// Process each element contained within the zip file. These can be
	// a mix of directories and files.
	var files []string
	for _, f := range reader.File {
		err := unzipFile(f, destination)
		if err != nil {
			return nil, err
		}
		if !f.FileInfo().IsDir() {
			files = append(files, filepath.Join(destination, f.Name))
		}
	}

	return files, nil
}

func unzipFile(f *zip.File, destination string) error {
	// Guard against "zip slip" vulnerability.
	filePath := filepath.Join(destination, f.Name)
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// If element is a directory, create it in the destination tree and return.
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	// For a file element, make sure its path exists.
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// Create a destination file to store the unzip contents, keeping the same mode as the original.
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Unzip the compressed file by copying its contents to the destination file.
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}
	return nil
}
