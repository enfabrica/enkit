package kbuildbarn

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// ByteStreamUrl retieves the CAS id and bytes of an action based on the input url.
// For example, bytestream://build.local.enfabrica.net:8000/blobs/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465
// should return (a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a, 465, nil)
// which is a BuildBarnParams.Hash, BuildBarnParams.Size, error.Error
func ByteStreamUrl(byteStream string) (string, string, error) {
	u, err := url.Parse(byteStream)
	if err != nil {
		return "", "", err
	}
	splitUrl := strings.Split(u.Path, "/")
	if len(splitUrl) != 4 {
		return "", "", fmt.Errorf("ByteStreamUrl() bytestream url is not well formed %s", byteStream)
	}
	return splitUrl[2], splitUrl[3], nil
}

// BuildBarnParams are the parameter necessary to reverse proxy to a bb_browser instance.
type BuildBarnParams struct {
	FileName     string
	Hash         string
	Size         string
	InvocationID string

	// This is the base Url
	BaseUrl string
	Scheme  string

	// These are the default buildbarn templates for their different types inside the CAS
	FileTemplate      string
	ActionTemplate    string
	CommandTemplate   string
	DirectoryTemplate string
}

// the following default values are arbitrary, based on what current works with buildbarn
var (
	DefaultFileTemplate      = "/blobs/file/%s-%s/%s"
	DefaultActionTemplate    = "/blobs/action/%s-%s/"
	DefaultCommandTemplate   = "/blobs/command/%s-%s"
	DefaultDirectoryTemplate = "/blobs/directory/%s-%s/"
)

func NewBuildBarnParams(baseUrl, fileName, hash, size string) *BuildBarnParams {
	// These are prefilled defaults, we can change at will
	return &BuildBarnParams{
		Scheme:            "http",
		FileName:          fileName,
		BaseUrl:           baseUrl,
		Hash:              hash,
		Size:              size,
		FileTemplate:      DefaultFileTemplate,
		ActionTemplate:    DefaultActionTemplate,
		CommandTemplate:   DefaultCommandTemplate,
		DirectoryTemplate: DefaultDirectoryTemplate,
	}
}

func performRequest(client *http.Client, url string) (io.ReadCloser, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (bbp BuildBarnParams) FileUrl() string {
	u := &url.URL{
		Scheme: bbp.Scheme,
		Host:   bbp.BaseUrl,
		Path:   fmt.Sprintf(bbp.FileTemplate, bbp.Hash, bbp.Size, bbp.FileName),
	}
	return u.String()
}

func (bbp BuildBarnParams) ActionUrl() string {
	u := &url.URL{
		Scheme: bbp.Scheme,
		Host:   bbp.BaseUrl,
		Path:   fmt.Sprintf(bbp.ActionTemplate, bbp.Hash, bbp.Size),
	}
	return u.String()
}

func (bbp BuildBarnParams) DirectoryUrl() string {
	u := &url.URL{
		Scheme: bbp.Scheme,
		Host:   bbp.BaseUrl,
		Path:   fmt.Sprintf(bbp.DirectoryTemplate, bbp.Hash, bbp.Size),
	}
	return u.String()
}

func (bbp BuildBarnParams) CommandUrl() string {
	u := &url.URL{
		Scheme: bbp.Scheme,
		Host:   bbp.BaseUrl,
		Path:   fmt.Sprintf(bbp.CommandTemplate, bbp.Hash, bbp.Size),
	}
	return u.String()
}

// RetryUntilSuccess just blasts through all possible urls until it hits one that works. This is intended for
// applications that are blind to the type of artifact
func RetryUntilSuccess(params BuildBarnParams) ([]byte, error) {
	urls := []string{
		params.FileUrl(), params.ActionUrl(), params.CommandUrl(), params.DirectoryUrl(),
	}
	var errs []error
	for _, uri := range urls {
		rc, err := performRequest(http.DefaultClient, uri)
		if err != nil {
			errs = append(errs)
			continue
		}
		return ioutil.ReadAll(rc)
	}
	return nil, multierror.New(errs)
}
