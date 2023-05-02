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

	// BUG(INFRA-1836): When bazel is talking to the BES endpoint via UNIX domain
	// socket, it embeds bytestream URLs that have:
	// * an excessive number of slashes (bytestream://////rest/of/path)
	// * no host - only a path
	// * path is absolute path to the UDS on the client joined with the normal
	//   path (/blobs/hash/size)
	// If this is the case, truncate path to what it otherwise would be when
	// talking to a web endpoint (/blobs/hash/size, with leading slash)
	const sockSuffix = ".sock"
	if strings.Contains(u.Path, "////") && strings.Contains(u.Path, sockSuffix) {
		idx := strings.Index(u.Path, sockSuffix)
		u.Path = u.Path[idx+len(sockSuffix):]
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
	return resp.Body, nil
}

func Url(baseName, hashFn, hash, size string, opts ...Option) string {
	cfg := generateOptions(baseName, hashFn, hash, size, opts...)
	u := &url.URL{
		Scheme: cfg.Scheme,
		Host:   baseName,
		Path:   fmt.Sprintf(cfg.PathTemplate, cfg.TemplateArgs...),
	}
	return u.String()
}

func File(baseName, hashFn, hash, size string, opts ...Option) string {
	cfg := generateOptions(baseName, hashFn, hash, size, opts...)
	return filepath.Join(baseName, fmt.Sprintf(cfg.PathTemplate, cfg.TemplateArgs...))
}

func readAndClose(rc io.ReadCloser) ([]byte, error) {
	// normally we do defer log.ErrorIfNotNull(DeferFunc()) but that isn't plumbed yet
	defer func() { _ = rc.Close() }()
	return ioutil.ReadAll(rc)
}

// RetryUntilSuccess just blasts through all possible urls until it hits one that works. This is intended for
// applications that are blind to the type of artifact
func RetryUntilSuccess(baseName, hashFn, hash, size string) ([]byte, error) {
	urls := []string{
		Url(baseName, hashFn, hash, size, WithActionUrlTemplate()),
		Url(baseName, hashFn, hash, size, WithDirectoryUrlTemplate()),
		Url(baseName, hashFn, hash, size, WithCommandUrlTemplate()),
		Url(baseName, hashFn, hash, size, WithByteStreamTemplate()),
	}
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
