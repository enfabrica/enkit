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
	"sync"
	"sync/atomic"
	"time"
)

const (
	fetchBufferSize = 64 * 1024 // 64Kb
)

type AssetDownloader interface {
	FetchItem(uri string, headers http.Header, expectedHash string) (*pb.Digest, error)
}

type downloadResult struct {
	digest *pb.Digest
	err    error
}

type downloadItem struct {
	uri          string
	headers      http.Header
	expectedHash string
	mutex        sync.Mutex
	finished     *downloadResult
	observers    []chan *downloadResult
}

type assetDownloader struct {
	cache             CacheProxy
	filter            UrlFilter
	metrics           Metrics
	parallelDownloads int32
	active            atomic.Int32
	queued            atomic.Int64
	currentMutex      sync.Mutex
	currentDownloads  map[string]*downloadItem
	downloadsQueue    chan *downloadItem
	accessLogger      *log.Logger
	errorLogger       *log.Logger
}

func NewAssetDownloader(config Config, cache CacheProxy, filter UrlFilter, metrics Metrics) AssetDownloader {
	return &assetDownloader{
		cache:             cache,
		filter:            filter,
		metrics:           metrics,
		parallelDownloads: config.ParallelDownloads(),
		active:            atomic.Int32{},
		queued:            atomic.Int64{},
		currentMutex:      sync.Mutex{},
		currentDownloads:  make(map[string]*downloadItem),
		downloadsQueue:    make(chan *downloadItem),
		accessLogger:      config.AccessLogger(),
		errorLogger:       config.ErrorLogger(),
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
		buf := make([]byte, fetchBufferSize)
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

func (ad *assetDownloader) fetchAsset(ctx context.Context, uri string, headers http.Header, expectedHash string) (*pb.Digest, error) {
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

func (ad *assetDownloader) addItemToQueue(item *downloadItem) {
	ad.queued.Add(1)
	if ad.active.Add(1) > ad.parallelDownloads {
		ad.active.Add(-1)
		ad.downloadsQueue <- item
		return
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer ad.active.Add(-1)

		for {
			ad.queued.Add(-1)
			ad.metrics.OnFetchStarted()
			digest, err := ad.fetchAsset(ctx, item.uri, item.headers, item.expectedHash)
			result := &downloadResult{
				digest: digest,
				err:    err,
			}

			item.mutex.Lock()
			item.finished = result
			item.mutex.Unlock()

			for _, observer := range item.observers {
				observer <- result
			}

			ad.currentMutex.Lock()
			delete(ad.currentDownloads, item.uri)
			ad.currentMutex.Unlock()

			for {
				select {
				case item = <-ad.downloadsQueue:
					break
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					if ad.queued.Load() == 0 {
						return
					}
				}
			}
		}
	}()
}

func (ad *assetDownloader) scheduleFetch(uri string, headers http.Header, expectedHash string) chan *downloadResult {
	ad.metrics.OnFetchRequested()

	result := make(chan *downloadResult, 1)
	item := &downloadItem{
		uri:          uri,
		headers:      headers,
		expectedHash: expectedHash,
		mutex:        sync.Mutex{},
		finished:     nil,
		observers:    []chan *downloadResult{result},
	}

	ad.currentMutex.Lock()
	defer ad.currentMutex.Unlock()

	existingItem, ok := ad.currentDownloads[uri]
	if ok && existingItem != nil {
		existingItem.mutex.Lock()
		existingItem.mutex.Unlock()

		if existingItem.finished != nil {
			result <- existingItem.finished
		} else {
			existingItem.observers = append(existingItem.observers, result)
		}
	} else {
		ad.currentDownloads[uri] = item
		defer ad.addItemToQueue(item)
	}

	return result
}

func (ad *assetDownloader) FetchItem(uri string, headers http.Header, expectedHash string) (*pb.Digest, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to parse URI: %s err: %v", uri, err)
	}

	if !ad.filter.CanProceed(u) {
		ad.accessLogger.Printf("GRPC ASSET %s FILTERED, SKIPPING", uri)
		return nil, nil
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, status.Errorf(codes.Internal, "unsupported URI: %s", uri)
	}

	scheduleChan := ad.scheduleFetch(uri, headers, expectedHash)
	result := <-scheduleChan
	return result.digest, result.err
}
