package astore

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"context"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
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
	client astore.AstoreClient
}

func New(conn grpc.ClientConnInterface) *Client {
	client := astore.NewAstoreClient(conn)
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

type DownloadOptions struct {
	*client.CommonOptions
	*Options

	Output    string
	Overwrite bool
	// First architecture found is downloaded.
	Architecture []string
}

type PathType string

const (
	IdAuto PathType = "auto"
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

func GetPathType(name string, defaultId PathType) PathType {
	id := defaultId
	if id == IdAuto {
		if IsUid(name) {
			id = IdUid
		} else {
			id = IdPath
		}
	}
	return id
}

func RetrieveRequestFromPath(name string, defaultId PathType) (*astore.RetrieveRequest, PathType) {
	id := GetPathType(name, defaultId)

	req := &astore.RetrieveRequest{}
	switch id {
	case IdPath:
		req.Path = name
	case IdUid:
		req.Uid = name
	}
	return req, id
}

// GetRetrieveResponse performs a Retrieve request, and returns both the generated request, and returned response.
func (c *Client) GetRetrieveResponse(name string, archs []string, defaultId PathType) (*astore.RetrieveResponse, *astore.RetrieveRequest, PathType, error) {
	req, id := RetrieveRequestFromPath(name, defaultId)

	var response *astore.RetrieveResponse
	var err error
	for _, arch := range archs {
		req.Architecture = arch

		response, err = c.client.Retrieve(context.TODO(), req)
		if err == nil {
			break
		}
		if status.Code(err) != codes.NotFound {
			return nil, nil, id, client.NiceError(err, "Could not contact the metadata server. Is your connectivity working? Is the server up?\nFor debugging: %s", err)
		}
	}
	if err != nil {
		return nil, nil, id, status.Errorf(codes.NotFound, "Could not find package suitable for architecture %s - %s", archs, err)
	}
	return response, req, id, nil
}

func (c *Client) Download(names []string, defaultId PathType, o DownloadOptions) error {
	defaultOutputDir := ""
	defaultOutputFile := ""
	if len(names) > 1 || (len(o.Output) > 1 && o.Output[len(o.Output)-1] == os.PathSeparator) {
		if err := os.MkdirAll(o.Output, 0770); err != nil {
			return err
		}
		defaultOutputDir = o.Output
	} else {
		stat, err := os.Stat(o.Output)
		if err == nil && stat.IsDir() {
			defaultOutputDir = o.Output
		} else {
			defaultOutputFile = o.Output
		}
	}

	for _, name := range names {
		response, _, id, err := c.GetRetrieveResponse(name, o.Architecture, defaultId)
		if err != nil {
			return err
		}

		outputFile := defaultOutputFile
		if id == IdPath && outputFile == "" {
			outputFile = filepath.Base(name)
		}

		o.Formatter.Artifact(response.Artifact)

		p := o.Progress()
		if response.Url == "" {
			return fmt.Errorf("Invalid empty URL returned by server")
		}

		if outputFile == "" {
			outputFile = filepath.Base(response.Path)
			if outputFile == "" {
				return fmt.Errorf("Invalid / unknown output file used.")
			}
		}

		output := filepath.Join(defaultOutputDir, outputFile)
		shortpath := o.ShortPath(output)
		// FIXME: use a temporary filename, then move in place.
		p.Step("%s: creating file", shortpath)

		flags := os.O_CREATE | os.O_WRONLY | os.O_SYNC
		if o.Overwrite {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_EXCL
		}

		f, err := os.OpenFile(output, flags, 0660)
		if err != nil {
			return err
		}

		p.Step("%s: downloading", shortpath)
		if err := Download(context.TODO(), progress.WriterCreator(p, f), response.Url); err != nil {
			return err
		}
		p.Done()
	}
	return nil
}

type UploadOptions struct {
	*client.CommonOptions
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

func (c *Client) Upload(files []FileToUpload, o UploadOptions) ([]*astore.Artifact, error) {
	artifacts := []*astore.Artifact{}
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
		response, err := c.client.Store(context.TODO(), &astore.StoreRequest{})
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
			resp, err := c.client.Commit(context.TODO(), &astore.CommitRequest{
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
		return "", "", kflags.NewUsageError(fmt.Errorf("cannot specify -d and -f at the same time - either -d or -f must be used"))
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

type Options struct {
	Formatter
}

type ListOptions struct {
	*client.CommonOptions
	Tag []string
}

func (c *Client) List(path string, o ListOptions) ([]*astore.Artifact, []*astore.Element, error) {
	resp, err := c.client.List(context.TODO(), &astore.ListRequest{
		Path: path,
		Tag:  &astore.TagSet{Tag: o.Tag},
	})

	if err != nil {
		return nil, nil, client.NiceError(err, "list command failed - %s", err)
	}
	return resp.Artifact, resp.Element, nil
}
