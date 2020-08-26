package astore

import (
	"cloud.google.com/go/datastore"
	"context"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) Note(ctx context.Context, req *astore.NoteRequest) (*astore.NoteResponse, error) {
	if req.Uid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request - no sid and no path")
	}

	arts := []*astore.Artifact{}
	err := retry.New(retry.WithDescription("note transaction"), retry.WithLogger(s.options.logger)).Run(func() error {
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

			art.Note = req.Note
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

	return &astore.NoteResponse{Artifact: arts}, err
}
