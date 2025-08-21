package main

import (
	"context"
	"fmt"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	"os"
	"github.com/enfabrica/enkit/experimental/remote_asset_service/asset_service"
)

type testCtx struct {
	ctx             context.Context
	proxyCache      asset_service.CacheProxy
	assetDownloader asset_service.AssetDownloader
}

func (t *testCtx) checkContains(digest *pb.Digest) {
	containsDigest, err := t.proxyCache.Contains(t.ctx, digest.Hash)
	if err != nil {
		panic(err)
	}

	if !proto.Equal(containsDigest, digest) {
		panic(fmt.Sprintf("containsDigest: %s != digest: %s", containsDigest.String(), digest.String()))
	}
}

func (t *testCtx) downloadAndCheck(expectedHash string) {
	uuid := uuid.New().String()
	file, err := t.proxyCache.GetToFile(t.ctx, uuid, expectedHash)
	if file != nil {
		os.Remove(file.Name())
	} else {
		panic("not found")
	}
	if err != nil {
		panic(err)
	}
}

func (t *testCtx) runWithHash(uri string, expectedHash string) {
	digest, err := t.assetDownloader.FetchItem(t.ctx, uri, http.Header{}, expectedHash)
	if err != nil {
		panic(err)
	}

	if digest.Hash != expectedHash {
		panic(fmt.Sprintf("digest.Hash: %s != expectedHash: %s", digest.Hash, expectedHash))
	}

	t.checkContains(digest)
	t.downloadAndCheck(expectedHash)
}

func (t *testCtx) runWithoutHash(uri string, expectedHash string) {
	digest, err := t.assetDownloader.FetchItem(t.ctx, uri, http.Header{}, "")
	if err != nil {
		panic(err)
	}

	if digest.Hash != expectedHash {
		panic(fmt.Sprintf("digest.Hash: %s != expectedHash: %s", digest.Hash, expectedHash))
	}

	t.checkContains(digest)
	t.downloadAndCheck(expectedHash)
}

func run() error {
	proxyCache, err := asset_service.NewCacheProxy("127.0.0.1:8982")
	if err != nil {
		panic(err)
	}

	accessLogger := log.New(os.Stdout, "", log.LstdFlags)
	assetDownloader := asset_service.NewAssetDownloader(proxyCache, accessLogger)

	tCtx := &testCtx{
		ctx:             context.Background(),
		proxyCache:      proxyCache,
		assetDownloader: assetDownloader,
	}

	tCtx.runWithHash(
		"https://github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
		"bc283cdfcd526a52c3201279cda4bc298652efa898b10b4db0837dc51652756f",
	)
	tCtx.runWithoutHash(
		"https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
		"7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
	)

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}
