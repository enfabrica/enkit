package astore

import (
	"context"

	apb "github.com/enfabrica/enkit/astore/proto"
	"github.com/enfabrica/enkit/lib/client"
)

func (c *Client) Note(uid string, note string) ([]*apb.Artifact, error) {
	req := &apb.NoteRequest{Uid: uid, Note: note}

	resp, err := c.client.Note(context.TODO(), req)
	if err != nil {
		return nil, client.NiceError(err, "could not annotate uid %s", err)
	}
	return resp.Artifact, nil
}
