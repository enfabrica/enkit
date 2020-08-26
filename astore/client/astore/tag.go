package astore

import (
	"context"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
)

type TagModifier func(*astore.TagRequest)

func TagSet(set []string) TagModifier {
	return func(tr *astore.TagRequest) {
		tr.Set = &astore.TagSet{Tag: set}
	}
}

func TagAdd(set []string) TagModifier {
	return func(tr *astore.TagRequest) {
		tr.Add = &astore.TagSet{Tag: set}
	}
}

func TagDel(set []string) TagModifier {
	return func(tr *astore.TagRequest) {
		tr.Del = &astore.TagSet{Tag: set}
	}
}

func (c *Client) Tag(uid string, mods ...TagModifier) ([]*astore.Artifact, error) {
	req := &astore.TagRequest{Uid: uid}

	for _, m := range mods {
		m(req)
	}

	resp, err := c.client.Tag(context.TODO(), req)
	if err != nil {
		return nil, client.NiceError(err, "could not tag uids %s", err)
	}
	return resp.Artifact, nil
}
