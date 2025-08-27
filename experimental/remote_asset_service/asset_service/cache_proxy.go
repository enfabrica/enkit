package asset_service

// Based on https://github.com/buildbarn/bb-storage/blob/master/pkg/grpc/base_client_factory.go

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-storage/pkg/clock"
	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/buildbarn/bb-storage/pkg/util"
	bs "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/security/advancedtls"
	"google.golang.org/grpc/status"
	"io"
	"math"
	"os"

	bbgrpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/jmespath"
)

const (
	// The maximum chunk size to write back to the client in Send calls.
	// Inspired by Goma's FileBlob.FILE_CHUNK maxium size.
	maxChunkSize = 2 * 1024 * 1024 // 2M
)

type CacheProxy interface {
	IsMissing(ctx context.Context, digest *pb.Digest) (bool, error)
	Contains(ctx context.Context, hash string) (*pb.Digest, error)
	Put(ctx context.Context, uuid string, digest *pb.Digest, rc io.ReadCloser) error
	GetToFile(ctx context.Context, uuid string, hash string) (*os.File, error)
}

type cacheProxy struct {
	ac  pb.ActionCacheClient
	cas pb.ContentAddressableStorageClient
	bs  bs.ByteStreamClient
	cap pb.CapabilitiesClient
}

type ClientInterceptor struct {
	metadataHeaderValues bbgrpc.MetadataHeaderValues
	metadataExtractor    bbgrpc.MetadataExtractor
}

func (client *ClientInterceptor) unaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if client.metadataExtractor != nil {
		extraMetadata, err := client.metadataExtractor(ctx)
		if err != nil {
			return util.StatusWrap(err, "Failed to extract metadata")
		}
		ctx = metadata.AppendToOutgoingContext(ctx, extraMetadata...)
	}

	ctx = metadata.AppendToOutgoingContext(ctx, client.metadataHeaderValues...)

	return invoker(ctx, method, req, reply, cc, opts...)
}

func (client *ClientInterceptor) streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if client.metadataExtractor != nil {
		extraMetadata, err := client.metadataExtractor(ctx)
		if err != nil {
			return nil, util.StatusWrap(err, "Failed to extract metadata")
		}
		ctx = metadata.AppendToOutgoingContext(ctx, extraMetadata...)
	}

	ctx = metadata.AppendToOutgoingContext(ctx, client.metadataHeaderValues...)

	return streamer(ctx, desc, cc, method, opts...)
}

func newClientDialOptionsFromTLSConfig(tlsConfig *tls.Config) ([]grpc.DialOption, error) {
	if tlsConfig == nil {
		return []grpc.DialOption{grpc.WithInsecure()}, nil
	}

	opts := advancedtls.Options{
		MinTLSVersion: tlsConfig.MinVersion,
		MaxTLSVersion: tlsConfig.MaxVersion,
		CipherSuites:  tlsConfig.CipherSuites,
		IdentityOptions: advancedtls.IdentityCertificateOptions{
			GetIdentityCertificatesForClient: tlsConfig.GetClientCertificate,
		},
		RootOptions: advancedtls.RootCertificateOptions{
			RootCertificates: tlsConfig.RootCAs,
		},
	}
	// advancedtls checks MinTLSVersion > MaxTLSVersion before applying
	// defaults:
	// https://github.com/grpc/grpc-go/blob/master/security/advancedtls/advancedtls.go#L243-L245
	// If setting a default minimum, set math.MaxUint16 as the Max to get around
	// this check.
	if opts.MaxTLSVersion == 0 {
		opts.MaxTLSVersion = math.MaxUint16
	}

	tc, err := advancedtls.NewClientCreds(&opts)
	if err != nil {
		return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Failed to configure GRPC client TLS")
	}
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(tc)}
	if tlsConfig.ServerName != "" {
		dialOptions = append(dialOptions, grpc.WithAuthority(tlsConfig.ServerName))
	}

	return dialOptions, nil
}

func NewCacheProxy(config CacheConfig, group program.Group) (CacheProxy, error) {
	address := config.ProxyAddress()

	var opts []grpc.DialOption
	if address.Scheme == "grpcs" {
		tlsFromConfig := config.TlsConfig()
		if tlsFromConfig != nil {
			tlsCredentials := credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			})
			opts = append(opts, grpc.WithTransportCredentials(tlsCredentials))
		} else {
			tlsConfig, err := util.NewTLSConfigFromClientConfiguration(tlsFromConfig)
			if err != nil {
				return nil, util.StatusWrap(err, "Failed to create TLS configuration")
			}
			tlsDialOpts, err := newClientDialOptionsFromTLSConfig(tlsConfig)
			if err != nil {
				return nil, util.StatusWrap(err, "Failed to convert TLS configuration")
			}
			opts = append(opts, tlsDialOpts...)
		}

	} else if address.Scheme == "grpc" {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "unknown address scheme: %s", address.Scheme)
	}

	metadataDefs := config.Metadata()
	jmesExpression := config.MetadataJmespathExpression()
	if len(metadataDefs) != 0 || jmesExpression != nil {
		var metadataHeaderValues bbgrpc.MetadataHeaderValues
		for _, entry := range metadataDefs {
			metadataHeaderValues.Add(entry.Header, entry.Values)
		}

		expr, err := jmespath.NewExpressionFromConfiguration(jmesExpression, group, clock.SystemClock)
		if err != nil {
			return nil, util.StatusWrap(err, "Failed to compile JMESPath expression")
		}

		metadataExtractor, err := bbgrpc.NewJMESPathMetadataExtractor(expr)
		if err != nil {
			return nil, util.StatusWrap(err, "Failed to create JMESPath extractor")
		}

		clientInterceptor := &ClientInterceptor{
			metadataHeaderValues: metadataHeaderValues,
			metadataExtractor:    metadataExtractor,
		}

		opts = append(opts, grpc.WithUnaryInterceptor(clientInterceptor.unaryInterceptor))
		opts = append(opts, grpc.WithStreamInterceptor(clientInterceptor.streamInterceptor))
	}

	conn, err := grpc.NewClient(address.Host, opts...)
	if err != nil {
		return nil, err
	}

	return &cacheProxy{
		ac:  pb.NewActionCacheClient(conn),
		cas: pb.NewContentAddressableStorageClient(conn),
		bs:  bs.NewByteStreamClient(conn),
		cap: pb.NewCapabilitiesClient(conn),
	}, nil
}

func actionDigest(hash string) *pb.Digest {
	h := sha256.New()

	h.Write([]byte(hash))
	h.Write([]byte("Action Cache Salt"))

	hashBytes := h.Sum(nil)
	hashStr := hex.EncodeToString(hashBytes[:])
	return &pb.Digest{Hash: hashStr}
}

func (cp *cacheProxy) IsMissing(ctx context.Context, digest *pb.Digest) (bool, error) {
	missingBlobs, err := cp.cas.FindMissingBlobs(ctx, &pb.FindMissingBlobsRequest{
		BlobDigests: []*pb.Digest{digest},
	})

	if err != nil {
		return true, status.Errorf(codes.Internal, "error on query FindMissingBlobs: %s", err)
	}

	switch len(missingBlobs.MissingBlobDigests) {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return true, status.Errorf(
			codes.DataLoss,
			"mailformmedd FindMissingBlobs response: len is %d",
			len(missingBlobs.MissingBlobDigests),
		)
	}
}

func (cp *cacheProxy) Contains(ctx context.Context, hash string) (*pb.Digest, error) {
	actionResult, err := cp.ac.GetActionResult(ctx, &pb.GetActionResultRequest{
		ActionDigest: actionDigest(hash),
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, status.Errorf(codes.Internal, "failed to query GetActionResult: %s", err)
	}

	if len(actionResult.OutputFiles) != 1 {
		return nil, status.Errorf(codes.DataLoss, "corrupted ActionResult: doesn't contain single output file")
	}

	assetDigest := actionResult.OutputFiles[0].Digest

	if assetDigest.Hash != hash {
		return nil, status.Errorf(
			codes.DataLoss,
			"corrupted ActionResult: hash of output file differs output file, re"+
				"quested: %s, but got: %s",
			hash,
			assetDigest.Hash,
		)
	}

	isMissing, err := cp.IsMissing(ctx, assetDigest)
	if err != nil {
		return nil, err
	}

	if isMissing {
		return nil, nil
	} else {
		return assetDigest, nil
	}
}

func streamError(stream bs.ByteStream_WriteClient, template string, err error) error {
	closeErr := stream.CloseSend()
	if closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	return status.Errorf(codes.Internal, template, err)
}

func (cp *cacheProxy) Put(ctx context.Context, uuid string, digest *pb.Digest, rc io.ReadCloser) error {
	// Query Capabilities to check this cache instance works
	serverCaps, err := cp.cap.GetCapabilities(context.Background(), &pb.GetCapabilitiesRequest{})
	if err != nil {
		return err
	}

	if !serverCaps.CacheCapabilities.ActionCacheUpdateCapabilities.UpdateEnabled {
		return status.Errorf(codes.PermissionDenied, "Cache update capabilities not enabled")
	}

	stream, err := cp.bs.Write(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to initialize Write stream: %s", err)
	}

	bufSize := digest.SizeBytes
	if bufSize > maxChunkSize {
		bufSize = maxChunkSize
	}

	buf := make([]byte, bufSize)

	template := "uploads/%s/blobs/%s/%d"
	resourceName := fmt.Sprintf(template, uuid, digest.Hash, digest.SizeBytes)
	firstIteration := true

	read := int64(0)
	offset := int64(0)
	for {
		n := 0
		for int64(n) < bufSize {
			nread, err := rc.Read(buf[n:])
			if err != nil && err != io.EOF {
				return streamError(stream, "failed to read asset data: %s", err)
			}
			if nread != 0 {
				n += nread
			} else {
				break
			}
		}

		if n > 0 {
			offset = read
			read += int64(n)
			finishWrite := read == digest.SizeBytes
			if read > digest.SizeBytes {
				return streamError(
					stream,
					"read more bytes than expected: %s",
					errors.New(fmt.Sprintf("expected: %d, got: %d", read, digest.SizeBytes)),
				)
			}

			rn := ""
			if firstIteration {
				firstIteration = false
				rn = resourceName
			}
			req := &bs.WriteRequest{
				ResourceName: rn,
				Data:         buf[:n],
				WriteOffset:  offset,
				FinishWrite:  finishWrite,
			}
			err := stream.Send(req)
			if err != nil && err != io.EOF {
				if err == io.EOF {
					break
				}
				return streamError(stream, "failed to send stream: %s", err)
			}
		} else {
			_, err = stream.CloseAndRecv()
			if err != nil {
				return err
			}
			break
		}
	}

	_, err = cp.ac.UpdateActionResult(ctx, &pb.UpdateActionResultRequest{
		ActionDigest: actionDigest(digest.Hash),
		ActionResult: &pb.ActionResult{
			OutputFiles: []*pb.OutputFile{{
				Digest: digest,
			}},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (cp *cacheProxy) GetToFile(ctx context.Context, uuid string, hash string) (*os.File, error) {
	digest, err := cp.Contains(ctx, hash)
	if err != nil {
		return nil, err
	} else if digest == nil {
		return nil, nil
	}

	template := "blobs/%s/%d"
	req := bs.ReadRequest{
		ResourceName: fmt.Sprintf(template, digest.Hash, digest.SizeBytes),
	}

	stream, err := cp.bs.Read(ctx, &req)
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-", uuid))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create temp file: %s", err)
	}

	isOk := false

	defer func() {
		if isOk {
			return
		}
		os.Remove(tmpFile.Name())
	}()

	write := int64(0)
	h := sha256.New()
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			err := stream.CloseSend()
			if err != nil {
				return nil, err
			}
			break
		} else if err != nil {
			return nil, err
		}

		h.Write(resp.GetData())
		n, err := tmpFile.Write(resp.GetData())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to write to temp file: %s", err)
		}
		write += int64(n)
	}

	if write != digest.SizeBytes {
		return nil, status.Errorf(codes.DataLoss, "downloaded size: %d, differ from requested: %d")
	}

	hashBytes := h.Sum(nil)
	hashStr := hex.EncodeToString(hashBytes[:])

	if hashStr != hash {
		return nil, status.Errorf(codes.DataLoss, ": %s", err)
	}

	isOk = true
	return tmpFile, nil
}
