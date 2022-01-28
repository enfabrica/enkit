package kbuildbarn

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// ParseByteStreamUrl retrieves  the CAS id and bytes of an action based on the input url.
// For example, bytestream://build.local.enfabrica.net:8000/blobs/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465
// should return (a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a, 465, nil)
// which is a BuildBarnParams.Hash, BuildBarnParams.Size, error.Error
func ParseByteStreamUrl(byteStream string) (string, string, error) {
	u, err := url.Parse(byteStream)
	if err != nil {
		return "", "", err
	}
	splitUrl := strings.Split(u.Path, "/")
	if len(splitUrl) != 4 {
		return "", "", fmt.Errorf("ParseByteStreamUrl() bytestream url is not well formed %s", byteStream)
	}
	return splitUrl[2], splitUrl[3], nil
}


func performRequest(client *http.Client, url string) (io.ReadCloser, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(resp.Body)
		defer func() { _ = resp.Body.Close() }()
		return nil, multierror.New([]error{fmt.Errorf("error response %s", respBody), err})
	}
	return resp.Body, nil
}

func Url(baseName, hash, size string, opts ...Option) string {
	cfg := generateOptions(baseName, hash, size, opts...)
	u := &url.URL{
		Scheme: cfg.Scheme,
		Host:   baseName,
		Path:   fmt.Sprintf(cfg.PathTemplate, cfg.TemplateArgs...),
	}
	return u.String()
}

func File(baseName, hash, size string, opts ...Option) string {
	cfg := generateOptions(baseName, hash, size, opts...)
	return filepath.Join(baseName, fmt.Sprintf(cfg.PathTemplate, cfg.TemplateArgs...))
}

func readAndClose(rc io.ReadCloser) ([]byte, error) {
	// normally we do defer log.ErrorIfNotNull(DeferFunc()) but that isn't plumbed yet
	defer func() { _ = rc.Close() }()
	return ioutil.ReadAll(rc)
}

// RetryUntilSuccess just blasts through all possible urls until it hits one that works. This is intended for
// applications that are blind to the type of artifact
func RetryUntilSuccess(urls []string ) ([]byte, error) {
	var errs []error
	for _, uri := range urls {
		rc, err := performRequest(http.DefaultClient, uri)
		if err != nil {
			errs = append(errs)
			continue
		}
		return readAndClose(rc)
	}
	return nil, multierror.New(errs)
}
