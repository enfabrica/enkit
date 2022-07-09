package astore

import (
	"context"

	apb "github.com/enfabrica/enkit/astore/proto"
	"github.com/enfabrica/enkit/lib/client"
)

type TagModifier func(*apb.TagRequest)

func TagSet(set []string) TagModifier {
	return func(tr *apb.TagRequest) {
		tr.Set = &apb.TagSet{Tag: set}
	}
}

func TagAdd(set []string) TagModifier {
	return func(tr *apb.TagRequest) {
		tr.Add = &apb.TagSet{Tag: set}
	}
}

func TagDel(set []string) TagModifier {
	return func(tr *apb.TagRequest) {
		tr.Del = &apb.TagSet{Tag: set}
	}
}

func (c *Client) Tag(uid string, mods ...TagModifier) ([]*apb.Artifact, error) {
	req := &apb.TagRequest{Uid: uid}

	for _, m := range mods {
		m(req)
	}

	resp, err := c.client.Tag(context.TODO(), req)
	if err != nil {
		return nil, client.NiceError(err, "could not tag uids %s", err)
	}
	return resp.Artifact, nil
}
