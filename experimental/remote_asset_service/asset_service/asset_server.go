package asset_service

// Based on https://github.com/buchgr/bazel-remote/blob/master/server/grpc_asset.go

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	asset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpc_status "google.golang.org/grpc/status"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type assetServer struct {
	cache           CacheProxy
	assetDownloader AssetDownloader
	accessLogger    *log.Logger
	errorLogger     *log.Logger
}

func RegisterAssetServer(config Config, server *grpc.Server, cache CacheProxy, assetDownloader AssetDownloader) {
	asset.RegisterFetchServer(server, &assetServer{
		cache:           cache,
		assetDownloader: assetDownloader,
		accessLogger:    config.AccessLogger(),
		errorLogger:     config.ErrorLogger(),
	})
}

var errNilFetchBlobRequest = grpc_status.Error(codes.InvalidArgument, "expected a non-nil *FetchBlobRequest")

func (s *assetServer) FetchBlob(ctx context.Context, req *asset.FetchBlobRequest) (*asset.FetchBlobResponse, error) {

	if req == nil {
		return nil, errNilFetchBlobRequest
	}

	var sha256Str string

	globalHeader := http.Header{}

	uriSpecificHeaders := make(map[int]http.Header)

	for _, q := range req.GetQualifiers() {
		if q == nil {
			return &asset.FetchBlobResponse{
				Status: &status.Status{
					Code:    int32(codes.InvalidArgument),
					Message: "unexpected nil qualifier in FetchBlobRequest",
				},
			}, nil
		}

		const QualifierHTTPHeaderPrefix = "http_header:"
		const QualifierHTTPHeaderUrlPrefix = "http_header_url:"

		if strings.HasPrefix(q.Name, QualifierHTTPHeaderPrefix) {
			key := q.Name[len(QualifierHTTPHeaderPrefix):]

			globalHeader[key] = strings.Split(q.Value, ",")
			continue
		} else if strings.HasPrefix(q.Name, QualifierHTTPHeaderUrlPrefix) {
			idxAndKey := q.Name[len(QualifierHTTPHeaderUrlPrefix):]
			parts := strings.Split(idxAndKey, ":")
			if len(parts) != 2 {
				s.errorLogger.Printf("invalid http_header_url qualifier: \"%s\"", idxAndKey)
				continue
			}

			uriIndex, err := strconv.Atoi(parts[0])
			if err != nil {
				s.errorLogger.Printf("failed to parse URI index as int: %s", err)
				continue
			}

			if uriIndex < 0 || uriIndex >= len(req.GetUris()) {
				s.errorLogger.Printf("URI index for header is out of range [0 - %d]: %d", len(req.GetUris())-1, uriIndex)
				continue
			}

			if _, found := uriSpecificHeaders[uriIndex]; !found {
				uriSpecificHeaders[uriIndex] = make(http.Header)
			}
			uriSpecificHeaders[uriIndex].Add(parts[1], q.Value)

			continue
		}

		if q.Name == "checksum.sri" && strings.HasPrefix(q.Value, "sha256-") {
			// Ref: https://developer.mozilla.org/en-US/docs/Web/Security/Subresource_Integrity

			b64hash := strings.TrimPrefix(q.Value, "sha256-")

			decoded, err := base64.StdEncoding.DecodeString(b64hash)
			if err != nil {
				s.errorLogger.Printf("failed to base64 decode \"%s\": %v",
					b64hash, err)
				continue
			}

			sha256Str = hex.EncodeToString(decoded)

			found, err := s.cache.Contains(ctx, sha256Str)

			if err != nil {
				s.errorLogger.Printf("failed to query  cache.Contains: %s", err)
			} else if found != nil {
				s.accessLogger.Printf("CACHE HIT %s/%d", found.Hash, found.SizeBytes)
				return &asset.FetchBlobResponse{
					Status:     &status.Status{Code: int32(codes.OK)},
					BlobDigest: found,
				}, nil
			}
		}
	}

	// Cache miss.
	md, _ := metadata.FromIncomingContext(ctx)

	for uriIndex, uri := range req.GetUris() {
		uriSpecificHeader := globalHeader.Clone()
		if header, found := uriSpecificHeaders[uriIndex]; found {
			for key, value := range header {
				uriSpecificHeader[key] = value
			}
		}

		digest, err := s.assetDownloader.FetchItem(uri, uriSpecificHeader, md, sha256Str)
		if err != nil {
			s.errorLogger.Printf("failed to fetch item \"%s\": %v", uri, err)
		}

		if digest != nil {
			return &asset.FetchBlobResponse{
				Status:     &status.Status{Code: int32(codes.OK)},
				BlobDigest: digest,
				Uri:        uri,
			}, nil
		}

		// Not a simple file. Not yet handled...
	}

	return &asset.FetchBlobResponse{
		Status: &status.Status{Code: int32(codes.NotFound)},
	}, nil
}

func (s *assetServer) FetchDirectory(context.Context, *asset.FetchDirectoryRequest) (*asset.FetchDirectoryResponse, error) {
	return nil, grpc_status.Errorf(codes.Unimplemented, "GetZstd not implemented")
}
