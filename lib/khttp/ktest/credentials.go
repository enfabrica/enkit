// +build !release

package ktest

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

//Check Credentials will walk runfiles until it reaches credentials.json and returns it
func CheckCredentials() (string, error) {
	var files []string
	root := os.Getenv("RUNFILES_DIR")
	if root == ""{
		return "", errors.New("RUNFILES_DIR is not set")
	}
	var credentialsString string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		if info.Name() == "credentials.json" {
			b, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			credentialsString = string(b)
		}
		return nil
	})
	return credentialsString, err
}

