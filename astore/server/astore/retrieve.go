package astore

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/oauth"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Functions mocked in unit tests
var storageSignedURL = storage.SignedURL

type authType int

const (
	// Unsupported; do not use
	AuthTypeNone authType = iota
	// Authentication is handled by credentials cookie + SSO
	AuthTypeOauth
	// Authentication is handled by JWT token parameter
	AuthTypeToken
)

func getSingleParam(v url.Values, keys ...string) string {
	for _, k := range keys {
		if val := v.Get(k); val != "" {
			return val
		}
	}
	return ""
}

func getListParam(v url.Values, keys ...string) []string {
	for _, k := range keys {
		if val, ok := v[k]; ok {
			return val
		}
	}
	return nil
}

// DownloadArtifact turns an http.Request into an astore.RetrieveRequest,
// executes it, and invokes the specified handler with the result. Authorization
// is enforced on the request based on the specified `authType`.
func (s *Server) DownloadArtifact(prefix string, ehandler DownloadHandler, auth authType, w http.ResponseWriter, r *http.Request) {
	upath := path.Clean(r.URL.Path)
	if !strings.HasPrefix(upath, prefix) {
		ehandler(upath, nil, status.Errorf(codes.InvalidArgument, "path %s does not start with the required prefix %s", upath, prefix), w, r)
		return
	}
	astorePath := strings.TrimPrefix(upath, prefix)

	params := r.URL.Query()
	arch := getSingleParam(params, "a", "arch")
	uid := getSingleParam(params, "u", "uid")
	tags := getListParam(params, "t", "tag")

	switch auth {
	default:
		s.options.logger.Errorf("auth type '%v' not supported by DownloadArtifact()", auth)
		ehandler(upath, nil, status.Errorf(codes.Unauthenticated, "unhandled auth type: %v", auth), w, r)
	case AuthTypeOauth:
		// Assume user has been authenticated at a higher level by this point
		break
	case AuthTypeToken:
		if token := params.Get("token"); token != "" {
			if err := s.validateToken(token, uid); err != nil {
				switch {
				default:
					s.options.logger.Errorf("Request for uid %q: token validation error: %v", uid, err)
					ehandler(upath, nil, status.Errorf(codes.Unauthenticated, "invalid token"), w, r)
					return
				}
			}
		} else {
			s.options.logger.Errorf("Request for uid %q: no token on request requiring token auth", uid)
			ehandler(upath, nil, status.Errorf(codes.Unauthenticated, "missing required token in request parameters"), w, r)
			return
		}
	}

	req := &astore.RetrieveRequest{}
	req.Path = astorePath
	req.Uid = uid
	req.Architecture = arch

	if len(tags) > 0 {
		req.Tag = &astore.TagSet{}
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			req.Tag.Tag = append(req.Tag.Tag, t)
		}
	} else if uid != "" {
		// Fetching by UID only must populate an empty tag set, as no tag set
		// implies a tag of "latest".
		req.Tag = &astore.TagSet{}
	}

	retr, err := s.Retrieve(r.Context(), req)
	if err != nil {
		s.options.logger.Errorf("DownloadArtifact failed (path=%q uid=%q arch=%q tags=%+v): %v", req.Path, req.Uid, req.Architecture, req.Tag, err)
	}
	ehandler(upath, retr, err, w, r)
}

func (s *Server) Retrieve(ctx context.Context, req *astore.RetrieveRequest) (*astore.RetrieveResponse, error) {
	if req.Uid == "" && req.Path == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request - no uid and no path")
	}

	if err := s.options.acls.IsAllowed(oauth.GetCredentials(ctx)); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "request denied by ACL - %s", err)
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
