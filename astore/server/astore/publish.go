package astore

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	apb "github.com/enfabrica/enkit/astore/proto"
	"github.com/enfabrica/enkit/lib/oauth"

	"cloud.google.com/go/datastore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func publishKeyFromPath(orig string) (string, string, *datastore.Key, error) {
	cleaned := filepath.ToSlash(path.Clean(strings.TrimSpace(orig)))
	if cleaned == "" || cleaned == "." {
		return "", "", nil, fmt.Errorf("%s results in empty cleaned after normalization", cleaned)
	}

	dir := path.Join("published", cleaned)

	var key *datastore.Key
	elements := strings.Split(dir, "/")
	if elements[0] == "" {
		elements = elements[1:]
	}
	for _, element := range elements[:len(elements)] {
		key = datastore.NameKey(KindPathElement, element, key)
	}
	return dir, cleaned, key, nil
}

func keyForPublished(key *datastore.Key) *datastore.Key {
	return datastore.NameKey(KindPublished, "published", key)
}

type DownloadHandler func(string, *apb.RetrieveResponse, error, http.ResponseWriter, *http.Request)

func (s *Server) getPublished(prefix string, w http.ResponseWriter, r *http.Request) (string, *Published, error) {
	upath := path.Clean(r.URL.Path)
	if !strings.HasPrefix(upath, prefix) {
		return upath, nil, status.Errorf(codes.InvalidArgument, "path %s does not start with the required prefix %s", upath, prefix)
	}

	keypath := strings.TrimPrefix(upath, prefix)
	_, _, pkey, err := publishKeyFromPath(keypath)
	if err != nil {
		return upath, nil, status.Errorf(codes.InvalidArgument, "path %s is invalid - results in empty path after cleanups", upath)
	}

	published := Published{}
	err = s.ds.Get(s.ctx, keyForPublished(pkey), &published)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			err = status.Errorf(codes.NotFound, "artifact not found")
		}
		return upath, nil, err
	}
	return keypath, &published, nil
}

func (s *Server) DownloadPublished(prefix string, ehandler DownloadHandler, w http.ResponseWriter, r *http.Request) {
	upath, pub, err := s.getPublished(prefix, w, r)
	if err != nil {
		ehandler(upath, nil, err, w, r)
		return
	}

	parms := r.URL.Query()

	arch := parms.Get("a")
	if arch == "" {
		arch = parms.Get("arch")
	}
	uid := parms.Get("u")
	if uid == "" {
		uid = parms.Get("uid")
	}

	req := pub.ToRetrieveRequest()
	if req.Architecture == "" {
		req.Architecture = arch
	}
	if req.Uid == "" {
		req.Uid = uid
	}

	retr, err := s.Retrieve(context.TODO(), req)
	ehandler(upath, retr, err, w, r)
}

type ListHandler func(string, *apb.ListResponse, error, http.ResponseWriter, *http.Request)

func (s *Server) ListPublished(prefix string, ehandler ListHandler, w http.ResponseWriter, r *http.Request) {
	upath, pub, err := s.getPublished(prefix, w, r)
	if err != nil {
		ehandler(upath, nil, err, w, r)
		return
	}

	req := pub.ToListRequest()
	retr, err := s.List(context.TODO(), req)
	ehandler(upath, retr, err, w, r)
}

func (s *Server) Publish(ctx context.Context, req *apb.PublishRequest) (*apb.PublishResponse, error) {
	creator := oauth.GetCredentials(ctx).Identity.GlobalName()

	if s.options.publishBaseURL == "" {
		return nil, status.Errorf(codes.Unavailable, "publish service has not been configured on the server")
	}

	dpath, cleaned, pkey, err := publishKeyFromPath(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "path %s is invalid - results in empty path after cleanups", req.Path)
	}

	muts := mutationsForKeyPath(dpath, pkey, creator)
	_, err = s.ds.Mutate(s.ctx, muts...)
	if err != nil && !alreadyExistsError(err) {
		return nil, err
	}

	published := FromListRequest(req.Select, &Published{
		Parent:  dpath,
		Creator: creator,
		Created: time.Now(),
	})

	_, err = s.ds.Mutate(s.ctx, datastore.NewInsert(keyForPublished(pkey), published))
	if err != nil {
		return nil, err
	}

	return &apb.PublishResponse{Url: s.options.publishBaseURL + cleaned}, nil
}

func (s *Server) Unpublish(ctx context.Context, req *apb.UnpublishRequest) (*apb.UnpublishResponse, error) {
	_, _, pkey, err := publishKeyFromPath(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "path %s is invalid - results in empty path after cleanups", req.Path)
	}

	err = s.ds.Delete(s.ctx, keyForPublished(pkey))
	if err != nil {
		return nil, err
	}

	return &apb.UnpublishResponse{}, nil
}
