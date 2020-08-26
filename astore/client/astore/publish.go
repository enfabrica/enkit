package astore

import (
	"context"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
)

type ToPublish struct {
	// Public location where to publish the artifact.
	Public string

	// Path of the artifact
	Path string
	Uid  string
	Tag  *[]string

	// An architecture to bind this path to.
	// If empty, the client will be able to select the architecture.
	Architecture  string
	NonExistentOK bool
}

func (c *Client) Publish(el ToPublish) (string, *astore.ListResponse, error) {
	req := &astore.ListRequest{
		Path:         el.Path,
		Uid:          el.Uid,
		Architecture: el.Architecture,
	}
	if el.Tag != nil {
		req.Tag = &astore.TagSet{Tag: *el.Tag}
	}

	meta, err := c.client.List(context.TODO(), req)
	if err != nil && (!el.NonExistentOK || status.Code(err) != codes.NotFound) {
		return "", nil, err
	}
	if err != nil {
		log.Printf("ignoring not-found error, as NonExistentOK was passed")
	}

	pub := &astore.PublishRequest{Path: el.Public, Select: req}
	resp, err := c.client.Publish(context.TODO(), pub)
	if err != nil {
		return "", nil, err
	}
	return resp.Url, meta, nil
}

func (c *Client) Unpublish(el string) error {
	ur := &astore.UnpublishRequest{Path: el}
	_, err := c.client.Unpublish(context.TODO(), ur)
	if err != nil {
		return client.NiceError(err, "Public mapping for %s was not removed.\nFor debugging: %s", ur.Path, err)
	}
	return err
}
