package asset_service

// Based on https://github.com/buchgr/bazel-remote/blob/master/server/grpc_asset.go

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

const (
	// The maximum chunk size to write back to the client in Send calls.
	// Inspired by Goma's FileBlob.FILE_CHUNK maxium size.
	maxChunkSize = 2 * 1024 * 1024 // 2M
)

type AssetDownloader interface {
	FetchItem(ctx context.Context, uri string, headers http.Header, expectedHash string) (*pb.Digest, error)
}

type assetDownloader struct {
	cache        CacheProxy
	accessLogger *log.Logger
}

func NewAssetDownloader(cache CacheProxy, accessLogger *log.Logger) AssetDownloader {
	return &assetDownloader{
		cache:        cache,
		accessLogger: accessLogger,
	}
}

func (ad *assetDownloader) fetchToTempFile(ctx context.Context, uuid string, uri string, rc io.ReadCloser) (*pb.Digest, error) {
	// We can't call Put until we know the hash and size.
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-", uuid))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create temporary file: %s", err)
	}

	defer os.Remove(tmpFile.Name()) // Called 2nd
	defer tmpFile.Close()

	read := int64(0)
	h := sha256.New()
	{
		defer rc.Close()
		buf := make([]byte, maxChunkSize)
		for {
			n, err := rc.Read(buf)
			if err != nil && err != io.EOF {
				return nil, status.Errorf(codes.Unavailable, "failed to read from uri: %s, err: %s", uri, err)
			}
			if n > 0 {
				read += int64(n)
				h.Write(buf[:n])
				_, err = tmpFile.Write(buf[:n])
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to write to temporary file: %s", err)
				}
			}
			if err == io.EOF {
				break
			}
		}
	}

	hashBytes := h.Sum(nil)
	hashStr := hex.EncodeToString(hashBytes[:])

	tmpFile.Seek(0, 0)

	err = ad.cache.Put(ctx, uuid, hashStr, read, tmpFile)
	if err != nil && err != io.EOF {
		return nil, err
	}

	ad.accessLogger.Printf("GRPC ASSET PUT TO PROXY CACHE SUCCESS %s %s/%d", uri, hashStr, read)

	return &pb.Digest{
		Hash:      hashStr,
		SizeBytes: read,
	}, nil
}

func (ad *assetDownloader) FetchItem(ctx context.Context, uri string, headers http.Header, expectedHash string) (*pb.Digest, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to parse URI: %s err: %v", uri, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		if u.Scheme == "file" {
			return nil, nil
		}
		return nil, status.Errorf(codes.Internal, "unsupported URI: %s", uri)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http.Request: %s err: %v", uri, err)
	}

	req.Header = headers

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to get URI: %s err: %v", uri, err)
	}
	defer func() { _ = resp.Body.Close() }()
	rc := resp.Body

	ad.accessLogger.Printf("GRPC ASSET FETCH %s %s", uri, resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, status.Errorf(codes.Unavailable, "failed to fetch http asset: %s", resp.Status)
	}

	uuid := uuid.New().String()
	expectedSize := resp.ContentLength
	if expectedHash == "" || expectedSize < 0 {
		// We can't call Put until we know the hash and size.
		return ad.fetchToTempFile(ctx, uuid, uri, rc)
	} else {
		err = ad.cache.Put(ctx, uuid, expectedHash, expectedSize, rc)
		if err != nil {
			return nil, err
		}

		ad.accessLogger.Printf("GRPC ASSET PUT TO PROXY CACHE SUCCESS %s %s/%d", uri, expectedHash, expectedSize)

		return &pb.Digest{
			Hash:      expectedHash,
			SizeBytes: expectedSize,
		}, nil
	}
}
