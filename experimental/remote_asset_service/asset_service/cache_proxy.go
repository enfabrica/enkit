package asset_service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	bs "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"os"
)

const (
	// The maximum chunk size to write back to the client in Send calls.
	// Inspired by Goma's FileBlob.FILE_CHUNK maxium size.
	maxChunkSize = 2 * 1024 * 1024 // 2M
)

type CacheProxy interface {
	Contains(ctx context.Context, hash string) (*pb.Digest, error)
	Put(ctx context.Context, uuid string, hash string, size int64, rc io.ReadCloser) error
	GetToFile(ctx context.Context, uuid string, hash string) (*os.File, error)
}

type cacheProxy struct {
	ac  pb.ActionCacheClient
	cas pb.ContentAddressableStorageClient
	bs  bs.ByteStreamClient
	cap pb.CapabilitiesClient
}

type ClientInterceptor struct {
	headers map[string]string
}

func (client *ClientInterceptor) unaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	for header, value := range client.headers {
		ctx = metadata.AppendToOutgoingContext(ctx, header, value)
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

func (client *ClientInterceptor) streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	for header, value := range client.headers {
		ctx = metadata.AppendToOutgoingContext(ctx, header, value)
	}

	return streamer(ctx, desc, cc, method, opts...)
}

func NewCacheProxy(config Config) (CacheProxy, error) {
	address := config.ProxyAddress()

	var opts []grpc.DialOption
	if address.Scheme == "grpcs" {
		creds := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else if address.Scheme == "grpc" {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "unknown address scheme: %s", address.Scheme)
	}

	headers := config.ProxyHeaders()
	if len(headers) != 0 {
		clientInterceptor := &ClientInterceptor{
			headers: headers,
		}

		opts = append(opts, grpc.WithUnaryInterceptor(clientInterceptor.unaryInterceptor))
		opts = append(opts, grpc.WithStreamInterceptor(clientInterceptor.streamInterceptor))
	}

	conn, err := grpc.NewClient(address.Host, opts...)
	if err != nil {
		return nil, err
	}

	caps := pb.NewCapabilitiesClient(conn)

	// Query Capabilities to check this cache instance works
	serverCaps, err := caps.GetCapabilities(context.Background(), &pb.GetCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	if !serverCaps.CacheCapabilities.ActionCacheUpdateCapabilities.UpdateEnabled {
		return nil, status.Errorf(codes.Unimplemented, "Cache update capabilities not enabled")
	}

	return &cacheProxy{
		ac:  pb.NewActionCacheClient(conn),
		cas: pb.NewContentAddressableStorageClient(conn),
		bs:  bs.NewByteStreamClient(conn),
		cap: caps,
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

	missingBlobs, err := cp.cas.FindMissingBlobs(ctx, &pb.FindMissingBlobsRequest{
		BlobDigests: []*pb.Digest{assetDigest},
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "error on query FindMissingBlobs: %s", err)
	}

	switch len(missingBlobs.MissingBlobDigests) {
	case 1:
		return nil, nil
	case 0:
		return assetDigest, nil
	default:
		return nil, status.Errorf(
			codes.DataLoss,
			"mailformmedd FindMissingBlobs response: len is %d",
			len(missingBlobs.MissingBlobDigests),
		)
	}
}

func streamError(stream bs.ByteStream_WriteClient, template string, err error) error {
	closeErr := stream.CloseSend()
	if closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	return status.Errorf(codes.Internal, template, err)
}

func (cp *cacheProxy) Put(ctx context.Context, uuid string, hash string, size int64, rc io.ReadCloser) error {
	stream, err := cp.bs.Write(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to initialize Write stream: %s", err)
	}

	bufSize := size
	if bufSize > maxChunkSize {
		bufSize = maxChunkSize
	}

	buf := make([]byte, bufSize)

	template := "uploads/%s/blobs/%s/%d"
	resourceName := fmt.Sprintf(template, uuid, hash, size)
	firstIteration := true

	read := int64(0)
	offset := int64(0)
	for {
		n, err := rc.Read(buf)
		if err != nil && err != io.EOF {
			return streamError(stream, "failed to read asset data: %s", err)
		}
		if n > 0 {
			offset = read
			read += int64(n)
			if read > size {
				return streamError(
					stream,
					"read more bytes than expected: %s",
					errors.New(fmt.Sprintf("expected: %d, got: %d", read, size)),
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
				FinishWrite:  read == size,
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
		ActionDigest: actionDigest(hash),
		ActionResult: &pb.ActionResult{
			OutputFiles: []*pb.OutputFile{{
				Digest: &pb.Digest{
					Hash:      hash,
					SizeBytes: size,
				},
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
