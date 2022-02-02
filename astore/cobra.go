package astore

import (
	"bytes"
	"context"
	"github.com/enfabrica/enkit/astore/client/astore"
	astorepb "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/progress"
	"github.com/spf13/viper"
	"strings"
)

type FileFetcher interface {
	FileContent(ctx context.Context, name string) (string, error)
}

var _ FileFetcher = (*realFileFetcher)(nil)

type realFileFetcher struct {
	c astorepb.AstoreClient
}

func (r *realFileFetcher) FileContent(ctx context.Context, path string) (string, error) {
	f, err := r.c.Retrieve(ctx, &astorepb.RetrieveRequest{Path: path})
	if err != nil {
		return "", err
	}
	b := new(strings.Builder)
	h := progress.NewDiscard()
	if err := astore.Download(context.TODO(), progress.WriterCreator(h, knetwork.NopWriteCloser(b)), f.Url); err != nil {
		return "", err
	}
	return b.String(), err
}

func ReadConfig(v *viper.Viper, bf *client.BaseFlags, sf *client.ServerFlags, fileName string, obj interface{}) error {
	_, cookie, err := bf.IdentityCookie()
	if err != nil {
		return err
	}
	storeconn, err := sf.Connect(client.WithCookie(cookie))
	if err != nil {
		return err
	}
	fetcher := &realFileFetcher{c: astorepb.NewAstoreClient(storeconn)}
	content, err := fetcher.FileContent(context.Background(), fileName)
	if err != nil {
		return err
	}
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBufferString(content)); err != nil {
		return err
	}
	if err := v.Unmarshal(&obj); err != nil {
		return err
	}
	return nil
}
