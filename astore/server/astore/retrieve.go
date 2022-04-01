package astore

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/enfabrica/enkit/astore/rpc/astore"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// Functions mocked in unit tests
	storageSignedURL = storage.SignedURL
)

// DownalodArtifact turns an http.Request into an astore.RetrieveRequest, executes it, and invokes the specified handler with the result.
func (s *Server) DownloadArtifact(prefix string, ehandler DownloadHandler, w http.ResponseWriter, r *http.Request) {
	upath := path.Clean(r.URL.Path)
	if !strings.HasPrefix(upath, prefix) {
		ehandler(upath, nil, status.Errorf(codes.InvalidArgument, "path %s does not start with the required prefix %s", upath, prefix), w, r)
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
	tag := parms["t"]
	if len(tag) <= 0 {
		tag = parms["tag"]
	}

	req := &astore.RetrieveRequest{}
	req.Path = strings.TrimPrefix(upath, prefix)
	req.Uid = uid
	req.Architecture = arch

	if len(tag) > 0 {
		req.Tag = &astore.TagSet{}
		for _, t := range tag {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			req.Tag.Tag = append(req.Tag.Tag, t)
		}
	}

	retr, err := s.Retrieve(context.TODO(), req)
	ehandler(upath, retr, err, w, r)
}

func (s *Server) Retrieve(ctx context.Context, req *astore.RetrieveRequest) (*astore.RetrieveResponse, error) {
	if req.Uid == "" && req.Path == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request - no uid and no path")
	}

	reqarch := strings.TrimSpace(req.Architecture)

	var query *datastore.Query
	var err error
	if req.Path != "" {
		query, err = queryForPath(KindArtifact, req.Path, reqarch)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid path - %s", err)
		}
	} else {
		query = datastore.NewQuery(KindArtifact)
	}
	query = query.Limit(1)

	if req.Uid != "" {
		query = query.Filter("Uid = ", req.Uid)
	}

	tags := []string{"latest"}
	if req.Tag != nil {
		tags = req.Tag.Tag
	}
	for _, tag := range tags {
		query = query.Filter("Tag = ", tag)
	}

	var artifacts []*Artifact
	keys, err := s.ds.GetAll(s.ctx, query, &artifacts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error running query - %s", err)
	}
	if len(keys) != 1 || len(artifacts) != 1 {
		return nil, status.Errorf(codes.NotFound, "artifact not found (%d found)", len(artifacts))
	}

	artifact := artifacts[0]
	url, err := storageSignedURL(s.options.bucket, objectPath(artifact.Sid), s.options.ForSigning("GET"))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not generate download URL - %s", err)
	}

	resp := &astore.RetrieveResponse{
		Path:     keyToPath(keys[0]),
		Artifact: artifact.ToProto(keyToArchitecture(keys[0])),
		Url:      url,
	}
	return resp, nil
}
