package astore

import (
	"context"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"encoding/base32"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/retry"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	ctx context.Context

	rng *rand.Rand

	gcs *storage.Client
	bkt *storage.BucketHandle

	ds *datastore.Client

	options Options
}

// Why base32?
// - fixed length (base58 is not fixed length)
// - has no characters other than letters and numbers (base64 has a few symbols)
// - restricts to characters that are not easy to confuse (eg, no 1 i l, 0 o ...)
// - it's a multiple of the # of bits we use, so there's no padding (base64 is 6 bits, not a multiple)
var idEncoder = base32.NewEncoding("abcdefghijkmnopqrstuvwxyz2345678")

// GenerateSid generates a path where to store the file.
func GenerateSid(rng *rand.Rand) (string, error) {
	sid := make([]byte, 20) // 160 bits.
	_, err := rng.Read(sid)
	if err != nil {
		return "", err
	}
	encoded := idEncoder.EncodeToString(sid)
	return encoded[0:2] + "/" + encoded[2:4] + "/" + encoded[4:], nil
}

// GenerateUid generates a unique identifier for the metadata.
func GenerateUid(rng *rand.Rand) (string, error) {
	uid := make([]byte, 20) // 160 bits.
	_, err := rng.Read(uid)
	if err != nil {
		return "", err
	}
	return idEncoder.EncodeToString(uid), nil
}

func (s *Server) Store(ctx context.Context, req *astore.StoreRequest) (*astore.StoreResponse, error) {
	sid, err := GenerateSid(s.rng)
	if err != nil {
		return nil, fmt.Errorf("problems with secure prng - %w", err)
	}

	url, err := storage.SignedURL(s.options.bucket, objectPath(sid), s.options.ForSigning("PUT"))
	if err != nil {
		return nil, fmt.Errorf("could not sign the url - %w", err)
	}

	return &astore.StoreResponse{Sid: sid, Url: url}, nil
}

func parentPath(p string) string {
	p, _ = path.Split(p)
	return trimSlash(p)
}

func queryForPath(kind, path, arch string) (*datastore.Query, error) {
	path, akey, err := keyFromPath(path, arch)
	if err != nil {
		return nil, err
	}
	return datastore.NewQuery(kind).Filter("Parent = ", path).Order("-Created").Ancestor(akey), nil
}

func (s *Server) List(ctx context.Context, req *astore.ListRequest) (*astore.ListResponse, error) {
	// Two queries are necessary:
	//   1) To retrieve artifacts.
	//   2) To retrieve sub-paths.
	childFiles := []*PathElement{}
	queryPath, err := queryForPath(KindPathElement, req.Path, "")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid path - %s", err)
	}
	kf, err := s.ds.GetAll(s.ctx, queryPath, &childFiles)
	if err != nil {
		return nil, err
	}

	reqarch := strings.TrimSpace(req.Architecture)
	queryArtifact, err := queryForPath(KindArtifact, req.Path, reqarch)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid path - %s", err)
	}
	if req.Uid != "" {
		queryArtifact = queryArtifact.Filter("Uid = ", req.Uid)
	}

	tags := []string{"latest"}
	if req.Tag != nil {
		tags = req.Tag.Tag
	}
	for _, tag := range tags {
		queryArtifact = queryArtifact.Filter("Tag = ", tag)
	}

	childArtifacts := []*Artifact{}
	ka, err := s.ds.GetAll(s.ctx, queryArtifact, &childArtifacts)
	if err != nil {
		return nil, err
	}

	dirs := []*astore.Element{}
	for ix, file := range childFiles {
		k := kf[ix]
		dirs = append(dirs, &astore.Element{Name: k.Name, Created: file.Created.UnixNano(), Creator: file.Creator})
	}

	arts := []*astore.Artifact{}
	for ix, art := range childArtifacts {
		arts = append(arts, art.ToProto(keyToArchitecture(ka[ix])))
	}

	response := astore.ListResponse{
		Element:  dirs,
		Artifact: arts,
	}
	return &response, nil
}

func objectPath(sid string) string {
	return path.Join("upload", sid)
}

func Rollback(t **datastore.Transaction) {
	if *t == nil {
		return
	}

	(*t).Rollback()
	(*t) = nil
}

func Commit(t **datastore.Transaction) error {
	_, err := (*t).Commit()
	(*t) = nil
	return err
}

func (s *Server) Tag(ctx context.Context, req *astore.TagRequest) (*astore.TagResponse, error) {
	if req.Uid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request - no sid and no path")
	}

	arts := []*astore.Artifact{}
	err := retry.New(retry.WithDescription("tag transaction"), retry.WithLogger(s.options.logger)).Run(func() error {
		t, err := s.ds.NewTransaction(s.ctx)
		if err != nil {
			return err
		}
		defer Rollback(&t)

		query := datastore.NewQuery(KindArtifact).Filter("Uid = ", req.Uid).Transaction(t)
		var artifacts []*Artifact
		keys, err := s.ds.GetAll(s.ctx, query, &artifacts)
		if err != nil {
			return status.Errorf(codes.Internal, "error running query - %s", err)
		}
		if len(artifacts) == 0 {
			return retry.Fatal(status.Errorf(codes.NotFound, "no match for uid - %s", req.Uid))
		}

		// Found list of artifacts to update. This should be a single artifact, as UIDs should
		// be globally unique. Using a loop for defense in depth.
		muts := []*datastore.Mutation{}
		for ix, art := range artifacts {
			key := keys[ix]

			if req.Set != nil {
				art.Tag = req.Set.Tag
			}
			if req.Add != nil {
				art.Tag = append(art.Tag, req.Add.Tag...)
			}
			var del []string
			if req.Del != nil {
				del = req.Del.Tag
			}

			art.Tag = cleanUniqueDelete(art.Tag, del)
			m, err := s.deleteTagsMutation(t, key, art.Tag)
			if err != nil {
				return err
			}
			muts = append(muts, m...)
			muts = append(muts, datastore.NewUpdate(key, art))
			arts = append(arts, art.ToProto(keyToArchitecture(key)))
		}

		_, err = t.Mutate(muts...)
		if err != nil {
			return err
		}

		err = Commit(&t)
		if err != nil {
			return err
		}
		return nil
	})

	return &astore.TagResponse{Artifact: arts}, err
}

func keyToArchitecture(key *datastore.Key) string {
	cursor := key
	for cursor != nil {
		if cursor.Kind == KindPathElement {
			return ""
		}
		if cursor.Kind == KindArchitecture {
			return cursor.Name
		}

		cursor = cursor.Parent
	}
	return ""
}

func keyToPath(key *datastore.Key) string {
	cursor := key
	path := ""
	for cursor != nil {
		if cursor.Kind == KindPathElement && cursor.Name != "root" && cursor.Name != "" {
			path = cursor.Name + "/" + path
		}

		cursor = cursor.Parent
	}

	if path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

func keyForArtifact(key *datastore.Key) *datastore.Key {
	return datastore.IncompleteKey(KindArtifact, key)
}

// keyFromPath cleans and parses the supplied path to compute a key.
// Returns the final path - after cleaning - and the computed key.
func keyFromPath(orig, architecture string) (string, *datastore.Key, error) {
	dir := path.Clean(filepath.ToSlash(strings.TrimSpace(orig)))
	if dir == "." {
		dir = ""
	}
	dir = path.Join("root", dir)

	var key *datastore.Key
	elements := strings.Split(dir, "/")
	if elements[0] == "" {
		elements = elements[1:]
	}
	for _, element := range elements[:len(elements)] {
		key = datastore.NameKey(KindPathElement, element, key)
	}
	if architecture != "" {
		key = datastore.NameKey(KindArchitecture, architecture, key)
	}
	return dir, key, nil
}

func trimSlash(str string) string {
	return strings.TrimSuffix(str, "/")
}

// mutationsForKeyPath computes the mutations necessary to create the path of objects supplied.
// path and key should come from keyFromPath to guarantee consistency and format.
func mutationsForKeyPath(dir string, key *datastore.Key, creator string) []*datastore.Mutation {
	muts := []*datastore.Mutation{}

	cursor := key
	parent := trimSlash(dir)

	for cursor != nil {
		switch cursor.Kind {
		case KindArchitecture:
			muts = append(muts, datastore.NewInsert(cursor, &Architecture{
				Parent:  parent,
				Created: time.Now(),
				Creator: creator,
			}))

		case KindPathElement:
			parent, _ = path.Split(parent)
			parent = trimSlash(parent)

			muts = append(muts, datastore.NewInsert(cursor, &PathElement{
				Parent:  parent,
				Created: time.Now(),
				Creator: creator,
			}))
		}

		cursor = cursor.Parent
	}
	return muts
}

func cleanUniqueDeleteMap(tags []string, seen map[string]struct{}) []string {
	res := []string{}
	for _, t := range tags {
		_, found := seen[t]
		if found {
			continue
		}
		seen[t] = struct{}{}
		res = append(res, strings.TrimSpace(t))
	}
	return res
}

func indexStrings(els []string) map[string]struct{} {
	index := map[string]struct{}{}
	for _, t := range els {
		index[t] = struct{}{}
	}
	return index
}

func cleanUniqueDelete(tags, del []string) []string {
	seen := indexStrings(del)
	return cleanUniqueDeleteMap(tags, seen)
}

// cleanUnique returns a copy of the array with each tag appearing once, with spaces trimmed.
func cleanUnique(tags []string) []string {
	return cleanUniqueDelete(tags, nil)
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func removeTag(tags []string, tag string) []string {
	if !hasTag(tags, tag) {
		return tags
	}

	result := []string{}
	for _, t := range tags {
		if t == tag {
			continue
		}
		result = append(result, t)
	}
	return result
}

func alreadyExistsError(err error) bool {
	if status.Code(err) == codes.AlreadyExists {
		return true
	}

	merr, ok := err.(datastore.MultiError)
	if !ok {
		return false
	}
	for _, err := range merr {
		if status.Code(err) != codes.AlreadyExists {
			return false
		}
	}
	return true
}

// deleteTagsMutation computes the mutations necessary to make sure the supplied list of tags is not applied to any other artifact in the same path/architecture.
//
// key is the key of the artifact owning the tags supplied, or the key of the parent where the artifact is supposed to be stored.
// tags is the list of tags to be added to the specified artifact. Those tags need to be removed from any other artifact.
func (s *Server) deleteTagsMutation(t *datastore.Transaction, key *datastore.Key, tags []string) ([]*datastore.Mutation, error) {
	pkey := key
	if key.Kind == KindArtifact {
		pkey = key.Parent
	}

	type KeyArtifact struct {
		Key *datastore.Key
		Art *Artifact
	}
	muts := []*datastore.Mutation{}
	entries := map[int64]KeyArtifact{}

	// A tag can only live on one version of an artifact.
	//
	// The goal of the loop is to identify all other artifacts that have one of the tags specified
	// assigned.
	//
	// Note that one artifact can have multiple tags. Given that this code is run in a transaction,
	// every read of the same artifact will show all the tags already available.
	//
	// To modify the list correct, the code has to:
	// - remove all the tags at once, so if the same object is written multiple times, it is always
	//   written with the correct set of tags.
	// - use the entries map above to prevent multiple writes.
	for _, tag := range tags {
		query := datastore.NewQuery(KindArtifact).Ancestor(pkey).Filter("Tag = ", tag).Transaction(t)

		// If a tag can only live on one version, this loop is not necessary.
		// But better safe than sorry, especially with eventual consistency and so on.
		for it := s.ds.Run(s.ctx, query); ; {
			var artifact Artifact
			curk, err := it.Next(&artifact)
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			if curk.Equal(key) {
				continue
			}
			entries[curk.ID] = KeyArtifact{Key: curk, Art: &artifact}
		}
	}

	for _, ka := range entries {
		ka.Art.Tag = cleanUniqueDelete(ka.Art.Tag, tags)
		muts = append(muts, datastore.NewUpdate(ka.Key, ka.Art))
	}

	return muts, nil
}

func (s *Server) Commit(ctx context.Context, req *astore.CommitRequest) (*astore.CommitResponse, error) {
	creds := oauth.GetCredentials(ctx)
	if req.Sid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Must supply an sid")
	}
	if req.Path == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Must supply a path")
	}

	architecture := "all"
	if req.Architecture != "" {
		architecture = req.Architecture
	}

	opath := objectPath(req.Sid)
	attrs, err := s.bkt.Object(opath).Attrs(s.ctx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "SID %s is invalid - %s", opath, err)
	}

	uid, err := GenerateUid(s.rng)
	if err != nil {
		return nil, err
	}

	creator := creds.Identity.GlobalName()

	_, err = s.bkt.Object(opath).Update(s.ctx, storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"path":    req.Path,
			"uid":     uid,
			"creator": creator,
		},
	})
	if err != nil {
		return nil, err
	}

	path, pkey, err := keyFromPath(req.Path, architecture)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid path - %s", err)
	}
	muts := mutationsForKeyPath(path, pkey, creator)
	_, err = s.ds.Mutate(s.ctx, muts...)
	if err != nil && !alreadyExistsError(err) {
		return nil, err
	}

	tags := cleanUnique(append(req.Tag, "latest"))
	artifact := &Artifact{
		Uid:     uid,
		Sid:     req.Sid,
		MD5:     attrs.MD5,
		Size:    attrs.Size,
		Tag:     tags,
		Parent:  path,
		Creator: creator,
		Created: time.Now(),
		Note:    req.Note,
	}

	err = retry.New(retry.WithDescription("insert transaction"), retry.WithLogger(s.options.logger)).Run(func() error {
		t, err := s.ds.NewTransaction(s.ctx)
		if err != nil {
			return err
		}
		defer Rollback(&t)

		muts, err := s.deleteTagsMutation(t, pkey, tags)
		if err != nil {
			return err
		}

		muts = append(muts, datastore.NewInsert(keyForArtifact(pkey), artifact))

		_, err = t.Mutate(muts...)
		if err != nil {
			return err
		}
		err = Commit(&t)
		if err != nil {
			return err
		}
		return nil
	})
	return &astore.CommitResponse{Artifact: artifact.ToProto(architecture)}, err
}
