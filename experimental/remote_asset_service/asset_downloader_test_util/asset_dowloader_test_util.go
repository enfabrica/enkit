package main

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/enfabrica/enkit/experimental/remote_asset_service/asset_service"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	"os"
)

type testCtx struct {
	ctx             context.Context
	urlFilter       asset_service.UrlFilter
	metrics         asset_service.Metrics
	proxyCache      asset_service.CacheProxy
	assetDownloader asset_service.AssetDownloader
}

func newTestCtx(configStr string) *testCtx {
	config, err := asset_service.NewConfigFromStr(configStr)
	if err != nil {
		panic(err)
	}

	proxyCache, err := asset_service.NewCacheProxy(config)
	if err != nil {
		panic(err)
	}

	urlFilter := asset_service.NewUrlFilter(config)
	metrics := asset_service.NewMetrics()
	assetDownloader := asset_service.NewAssetDownloader(config, proxyCache, urlFilter, metrics)

	return &testCtx{
		ctx:             context.Background(),
		urlFilter:       urlFilter,
		metrics:         metrics,
		proxyCache:      proxyCache,
		assetDownloader: assetDownloader,
	}
}

func (t *testCtx) checkContains(digest *pb.Digest) error {
	containsDigest, err := t.proxyCache.Contains(t.ctx, digest.Hash)
	if err != nil {
		return err
	}

	if !proto.Equal(containsDigest, digest) {
		return errors.New(fmt.Sprintf("containsDigest: %s != digest: %s", containsDigest.String(), digest.String()))
	}

	return nil
}

func (t *testCtx) downloadAndCheck(expectedHash string) error {
	uuid := uuid.New().String()
	file, err := t.proxyCache.GetToFile(t.ctx, uuid, expectedHash)
	if err != nil {
		return err
	}
	if file != nil {
		os.Remove(file.Name())
	} else {
		return errors.New("not found")
	}
	return nil
}

func (t *testCtx) runWithHash(uri string, expectedHash string) error {
	digest, err := t.assetDownloader.FetchItem(uri, http.Header{}, expectedHash)
	if err != nil {
		return err
	}

	if digest == nil {
		return errors.New("not fetched")
	}

	if digest.Hash != expectedHash {
		return errors.New(fmt.Sprintf("digest.Hash: %s != expectedHash: %s", digest.Hash, expectedHash))
	}

	err = t.checkContains(digest)
	if err != nil {
		return err
	}

	return t.downloadAndCheck(expectedHash)
}

func (t *testCtx) runWithoutHash(uri string, expectedHash string) error {
	digest, err := t.assetDownloader.FetchItem(uri, http.Header{}, "")
	if err != nil {
		return err
	}

	if digest == nil {
		return errors.New("not fetched")
	}

	if digest.Hash != expectedHash {
		return errors.New(fmt.Sprintf("digest.Hash: %s != expectedHash: %s", digest.Hash, expectedHash))
	}

	err = t.checkContains(digest)
	if err != nil {
		return err
	}

	return t.downloadAndCheck(expectedHash)
}

const configRaw = `{
	cache: {
		address: "grpc://127.0.0.1:8982"
	},
}`

func run() error {
	tCtx := newTestCtx(configRaw)

	err := tCtx.runWithHash(
		"https://github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
		"bc283cdfcd526a52c3201279cda4bc298652efa898b10b4db0837dc51652756f",
	)

	if err != nil {
		return err
	}

	err = tCtx.runWithoutHash(
		"https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
		"7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
	)

	if err != nil {
		return err
	}

	if tCtx.metrics.NumberOfFetches() != 2 {
		return errors.New(fmt.Sprintf("NumberOfFetches %d != expected 2", tCtx.metrics.NumberOfFetches()))
	}

	if tCtx.metrics.NumberOfRequestedFetches() != 2 {
		return errors.New(fmt.Sprintf("NumberOfRequestedFetches %d != expected 2", tCtx.metrics.NumberOfFetches()))
	}

	return nil
}

func runDeduplicationCheck() error {
	tCtx := newTestCtx(configRaw)

	wg := new(errgroup.Group)

	for range 10 {
		wg.Go(func() error {
			return tCtx.runWithHash(
				"https://github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
				"bc283cdfcd526a52c3201279cda4bc298652efa898b10b4db0837dc51652756f",
			)
		})
		wg.Go(func() error {
			return tCtx.runWithoutHash(
				"https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
				"7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
			)
		})
	}

	err := wg.Wait()
	if err != nil {
		return err
	}

	if tCtx.metrics.NumberOfFetches() != 2 {
		return errors.New(fmt.Sprintf("NumberOfFetches %d != expected 2", tCtx.metrics.NumberOfFetches()))
	}

	if tCtx.metrics.NumberOfRequestedFetches() != 20 {
		return errors.New(fmt.Sprintf("NumberOfRequestedFetches %d != expected 20", tCtx.metrics.NumberOfFetches()))
	}

	return nil
}

const configWithFilterRaw = `{
	cache: {
		address: "grpc://127.0.0.1:8982"
	},
	url_filter: {
		skip_hosts: [
			"us-docker.pkg.dev",
		],
	},
}`

func runFilterUriCheck() error {
	tCtx := newTestCtx(configWithFilterRaw)

	err := tCtx.runWithHash(
		"https://us-docker.pkg.dev/v2/enfabrica-container-images/third-party-prod/distroless/base/golang/manifests/sha256:a4eefd667af74c5a1c5efe895a42f7748808e7f5cbc284e0e5f1517b79721ccb",
		"a4eefd667af74c5a1c5efe895a42f7748808e7f5cbc284e0e5f1517b79721ccb",
	)

	if err == nil {
		return errors.New("no error on fetch for 'us-docker.pkg.dev', expected to be filtered out")
	}

	if err.Error() != "not fetched" {
		return errors.New(fmt.Sprintf("expected 'not fetched' for 'us-docker.pkg.dev', got '%s'", err))
	}

	err = tCtx.runWithHash(
		"file:/home/gleb/develop/enkit/registry/modules/rules_python/enf-1.4.1/patches/rules_python.patch",
		"bc3b0c2916152348ef7d465f6025aedc530b5edc8b9da82617eb79531f783302",
	)

	if err == nil {
		return errors.New("no error on fetch for 'file:' url scheme, expected to be filtered out")
	}

	if err.Error() != "not fetched" {
		return errors.New(fmt.Sprintf("expected 'not fetched' for 'file:' url scheme, got '%s'", err))
	}

	return tCtx.runWithHash(
		"https://github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
		"bc283cdfcd526a52c3201279cda4bc298652efa898b10b4db0837dc51652756f",
	)
}

func main() {
	_ = godotenv.Load(".env")

	var err error

	err = run()
	if err != nil {
		log.Fatal(err)
	}

	err = runDeduplicationCheck()
	if err != nil {
		log.Fatal(err)
	}
	
	err = runFilterUriCheck()
	if err != nil {
		log.Fatal(err)
	}
}
