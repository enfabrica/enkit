package astore

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	apb "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	"github.com/enfabrica/enkit/lib/grpcwebclient"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/progress"

	"github.com/go-git/go-git/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	conn   grpc.ClientConnInterface
	client apb.AstoreClient
}

func New(conn grpc.ClientConnInterface) *Client {
	client := apb.NewAstoreClient(conn)
	return &Client{conn: conn, client: client}
}

func NewWeb(server string, mods ...gwc.Modifier) (*Client, error) {
	conn, err := gwc.New(server, mods...)
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

func NewNative(server string, mods ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(server, mods...)
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

type DownloadProcessor func(ctx *ccontext.Context, file FileToDownload, response *apb.RetrieveResponse, outputDir, outputFile string) error

type DownloadOptions struct {
	*ccontext.Context

	// Function invoked to actually perform the download.
	// If not specified, defaults to DefaultDownloadProcessor.
	Processor DownloadProcessor
}

type FileToDownload struct {
	// Name of the file on the remote system.
	Remote     string   // ok
	RemoteType PathType // ok

	// How we want the file to be named on the local filesystem.
	Local string
	// Overwrite the file if there already?
	Overwrite bool
	// First architecture found is downloaded.
	Architecture []string // ok
	// No tags means latest tag.
	Tag *[]string
}

type PathType string

const (
	IdAuto PathType = ""
	IdPath          = "path"
	IdUid           = "uid"
)

var UidRegex = regexp.MustCompile("^[a-z0-9]{32}$")

func IsUid(path string) bool {
	if len(path) != 32 {
		return false
	}
	return UidRegex.MatchString(path)
}

func GetPathType(name string, id PathType) PathType {
	if id == IdUid || id == IdPath {
		return id
	}
	if IsUid(name) {
		return IdUid
	}
	return IdPath
}

func RetrieveRequestFromPath(name string, id PathType) (*apb.RetrieveRequest, PathType) {
	req := &apb.RetrieveRequest{}
	id = GetPathType(name, id)
	switch id {
	case IdPath:
		req.Path = name
	case IdUid:
		req.Uid = name
	}
	return req, id
}

// GetRetrieveResponse performs a Retrieve request, and returns both the generated request, and returned response.
func (c *Client) GetRetrieveResponse(name string, archs []string, defaultId PathType, tags *[]string) (*apb.RetrieveResponse, *apb.RetrieveRequest, PathType, error) {
	req, id := RetrieveRequestFromPath(name, defaultId)

	adapt := func(err error) error {
		if status.Code(err) != codes.NotFound {
			return client.NiceError(err, "Could not contact the metadata server. Is your connectivity working? Is the server up?\nFor debugging: %s", err)
		}
		return status.Errorf(codes.NotFound, "Could not find package archs: %s - %s", archs, err)
	}

	var response *apb.RetrieveResponse
	var err error
	if id == IdUid {
		// Tags are purposedly ignored when fetching by UID.
		// Server assumes that no tags means "latest", so we must explicitly specify
		// an empty tag set.
		req.Tag = &apb.TagSet{}
		response, err = c.client.Retrieve(context.TODO(), req)
		if err != nil {
			return nil, nil, id, adapt(err)
		}
	} else {
		if tags != nil {
			req.Tag = &apb.TagSet{Tag: *tags}
		}

		if len(archs) == 0 {
			archs = []string{"all"}
		}

		for _, arch := range archs {
			req.Architecture = arch

			response, err = c.client.Retrieve(context.TODO(), req)
			if err == nil {
				break
			}
			if status.Code(err) != codes.NotFound {
				return nil, nil, id, adapt(err)
			}
		}

		if err != nil {
			return nil, nil, id, adapt(err)
		}
	}
	return response, req, id, nil
}

// DefaultDownloadProcessor is the default processor used to download files.
//
// It downloads the file to a temporary file with a simple get request (no multipart/parallel downloads)
// and then moves it to the final location with an atomic move while printing a progress bar.
func DefaultDownloadProcessor(ctx *ccontext.Context, file FileToDownload, response *apb.RetrieveResponse, outputDir, outputFile string) error {
	output := filepath.Join(outputDir, outputFile)
	shortpath := ctx.ShortPath(output)

	// Yes, this is racy. Who knows if someone will create the file before the
	// download is over, or if the file will be gone by then.
	// However, it would be a shame if we spent 30 mins downloading a file to
	// then discover that we cannot overwrite it.
	// This is just to be nice to the user.
	if !file.Overwrite {
		if _, err := os.Stat(output); err == nil {
			return os.ErrExist
		}
	}

	p := ctx.Progress()
	p.Step("%s: creating file", shortpath)
	f, err := ioutil.TempFile(outputDir, "."+outputFile+".*")
	if err != nil {
		return err
	}

	p.Step("%s: downloading", shortpath)
	if err := Download(context.TODO(), progress.WriterCreator(p, f), response.Url); err != nil {
		os.Remove(f.Name())
		return err
	}

	if err := os.Link(f.Name(), output); err != nil {
		if !os.IsExist(err) || !file.Overwrite {
			return fmt.Errorf("trying to store file as %s, failed with: %w", output, err)
		}
		if err := os.Rename(f.Name(), output); err != nil {
			return err
		}
	}

	os.Remove(f.Name())
	p.Done()
	return nil
}

func (c *Client) Download(files []FileToDownload, o DownloadOptions) ([]*apb.Artifact, error) {
	arts := []*apb.Artifact{}
	processor := o.Processor
	if processor == nil {
		processor = DefaultDownloadProcessor
	}

	for _, file := range files {
		response, _, id, err := c.GetRetrieveResponse(file.Remote, file.Architecture, file.RemoteType, file.Tag)
		if err != nil {
			return nil, err
		}

		arts = append(arts, response.Artifact)

		if response.Url == "" {
			return nil, fmt.Errorf("Invalid empty URL returned by server")
		}

		outputDir, outputFile := "", ""
		stat, err := os.Stat(file.Local)
		if err == nil && stat.IsDir() {
			outputDir = file.Local
			outputFile = ""
		} else {
			outputDir, outputFile = filepath.Split(file.Local)
			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0770); err != nil {
					return nil, err
				}
			}
		}

		if id == IdPath && outputFile == "" {
			outputFile = filepath.Base(file.Remote)
		}
		if outputFile == "" {
			outputFile = filepath.Base(response.Path)
			if outputFile == "" {
				return nil, fmt.Errorf("Invalid / unknown output file used.")
			}
		}

		if err := processor(o.Context, file, response, outputDir, outputFile); err != nil {
			return nil, err
		}
	}
	return arts, nil
}

type UploadOptions struct {
	*ccontext.Context
}

type FileToUpload struct {
	// Which file needs to be open on the local file system.
	Local string
	// How we want the file named on the remote file system.
	Remote string
	// If this file is geared toward a specific architecture.
	Architecture []string
	// User assigned note, nothing to see here, just a string.
	Note string
	// List of tags to apply to the file.
	Tag []string
}

func (c *Client) Upload(files []FileToUpload, o UploadOptions) ([]*apb.Artifact, error) {
	artifacts := []*apb.Artifact{}
	for _, file := range files {
		o.Logger.Infof("uploading '%s' as '%s'", file.Local, file.Remote)

		shortpath := o.ShortPath(file.Local)

		p := o.Progress()
		p.Step("%s: opening", shortpath)
		fd, err := os.Open(file.Local)
		if err != nil {
			// FIXME: Handle the case where the fd is a directory.
			return artifacts, err
		}
		defer fd.Close()

		p.Step("%s: allocating id", shortpath)
		response, err := c.client.Store(context.TODO(), &apb.StoreRequest{})
		if err != nil {
			return artifacts, client.NiceError(err, "could not initiate store request %s", err)
		}

		if response.Sid == "" || response.Url == "" {
			return artifacts, fmt.Errorf("invalid server response")
		}

		info, err := fd.Stat()
		if err != nil {
			return artifacts, fmt.Errorf("couldn't stat %s - %w", shortpath, err)
		}

		p.Step("%s: uploading", shortpath)
		if err := Upload(context.TODO(), p.Reader(fd, info.Size()), info.Size(), response.Url); err != nil {
			return artifacts, err
		}
		// FIXME partial failure. UNDO upload.

		archs := file.Architecture
		if len(archs) == 0 {
			archs = []string{"all"}
		}
		for _, arch := range archs {
			p.Step("%s: committing %s", shortpath, arch)
			resp, err := c.client.Commit(context.TODO(), &apb.CommitRequest{
				Sid:          response.Sid,
				Architecture: arch,
				Path:         strings.TrimPrefix(file.Remote, "/"),
				Note:         file.Note,
				Tag:          file.Tag,
			})
			if err != nil {
				return artifacts, client.NiceError(err, "commit failed - %s", err)
			}
			artifacts = append(artifacts, resp.Artifact)
		}
		p.Done()
	}
	return artifacts, nil
}

func Download(ctx context.Context, f func(int64) io.WriteCloser, url string) error {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request returned status code %d - %s", resp.StatusCode, resp.Status)
	}

	w := f(resp.ContentLength)
	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}

	return nil
}

func Upload(ctx context.Context, r io.ReadCloser, size int64, url string) error {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, r)
	if err != nil {
		return err
	}
	if size > 0 {
		req.ContentLength = size
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Upload to url:\n\t%s\nFailed: status %s", url, resp.Status)
	}

	// Flush and discard any reply. This is strictly not needed.
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

type SuggestOptions struct {
	// If Directory is set, the returned remote location will use the set directory + the name of the file.
	Directory string
	File      string

	// Files specified to upload can be in the form /path/to/local@/path/to/remote/, which takes precedence
	// over any other recommendation mechanism.
	//
	// If DisableAt is set to true, this manual override is disabled.
	DisableAt bool
	// If DisableGit is set to true, git will not be used to suggest a remote file name.
	DisableGit bool
	// If DisableBazel is set to true, bazel heuristics will not be used to suggest a remote file name.
	// DisableBazel bool

	// Allow absolute paths.
	AllowAbsolute bool
	// Allow a file name without directory.
	AllowSingleElement bool
}

func SuggestGitName(name string) (string, error) {
	repo, err := git.PlainOpenWithOptions(filepath.Dir(name), &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return "", err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	root := tree.Filesystem.Root()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	absName, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}

	destName, err := filepath.Rel(absRoot, absName)
	if err != nil {
		return "", err
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return "", err
	}

	var base string
remotes:
	for _, remote := range remotes {
		config := remote.Config()
		for _, url := range config.URLs {
			base = strings.TrimSuffix(path.Base(url), ".git")
			if base != "" {
				break remotes
			}
		}
	}

	if base == "" {
		base = filepath.Base(root)
	}

	return path.Join(base, destName), nil
}

func CleanRemote(name string) (string, error) {
	if os.PathSeparator != '/' {
		name = strings.ReplaceAll(name, string(os.PathSeparator), "/")
	}
	return path.Clean(name), nil
}

func SuggestRemote(name string, options SuggestOptions) (string, string, error) {
	name, remote, err := FindRemote(name, options)
	if err != nil {
		return "", "", err
	}

	remote, err = CleanRemote(remote)
	if err != nil {
		return "", "", err
	}

	if !options.AllowAbsolute {
		if path.IsAbs(remote) {
			return "", "", fmt.Errorf("'%s' is an absolute path - this is probably not how you want to name the file in our repository. "+
				"Use a relative path, @ notation, or one of the options to tweak the naming (see --help).", remote)
		}
	}

	if !options.AllowSingleElement {
		dir, _ := path.Split(remote)
		if dir == "" {
			return "", "", fmt.Errorf("'%s' is in the root of your repository? You probably do not want to upload your artifacts there. "+
				"Use options to specify in which directory to upload the file, or to override this check (see --help).", remote)
		}
	}

	return name, remote, err
}

func FindRemote(name string, options SuggestOptions) (string, string, error) {
	if options.File != "" && options.Directory != "" {
		return "", "", kflags.NewUsageErrorf("cannot specify -d and -f at the same time - either -d or -f must be used")
	}
	if !options.DisableAt {
		ix := strings.LastIndex(name, "@")
		if ix > 0 {
			if ix == len(name)-1 {
				return "", "", fmt.Errorf("@ at the end of the argument? @ notation requires a remote path specified after the @ symbol. 0 length paths are not allowed")
			}
			local := name[:ix]
			remote := name[ix+1:]

			if remote[len(remote)-1] == '/' || remote[len(remote)-1] == os.PathSeparator {
				base := filepath.Base(local)
				remote = path.Join(remote, base)
			}
			return local, remote, nil
		}
	}

	if options.Directory != "" {
		base := filepath.Base(name)
		remote := path.Join(options.Directory, base)
		return name, remote, nil
	}
	if options.File != "" {
		remote := options.File
		return name, remote, nil
	}

	if !options.DisableGit {
		remote, err := SuggestGitName(name)
		if err == nil {
			return name, remote, nil
		}
	}

	return name, name, nil
}

type ListOptions struct {
	*ccontext.Context
	Tag []string
}

func (c *Client) List(path string, o ListOptions) ([]*apb.Artifact, []*apb.Element, error) {
	resp, err := c.client.List(context.TODO(), &apb.ListRequest{
		Path: path,
		Tag:  &apb.TagSet{Tag: o.Tag},
	})

	if err != nil {
		return nil, nil, client.NiceError(err, "list command failed - %s", err)
	}
	return resp.Artifact, resp.Element, nil
}
