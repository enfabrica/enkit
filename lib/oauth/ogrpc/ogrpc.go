// Authenticates gRPC requests by using an oauth cookie.
//
// To authenticate gRPC requests, just use ogrpc.StreamInterceptor and ogrpc.UnaryInterceptor:
//
//     oauth, err := oauth.New(rng, ...)
//
//     grpcs := grpc.NewServer(
//         grpc.StreamInterceptor(ogrpc.StreamInterceptor(oauth, "/auth.Auth/")),
//         grpc.UnaryInterceptor(ogrpc.UnaryInterceptor(oauth, "/auth.Auth/")),
//     )
//
// This will ensure that all grpc requests, except those whose name starts with /auth.Auth/,
// have a valid authentication cookie.
//
// The authentication cookie can then be retrieved using oauth.GetCredentials() on the grpc context
// passed to your method.
//

package ogrpc

import (
	"context"
	"github.com/enfabrica/enkit/lib/oauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type GrpcInterceptor func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error

// GetCookieValue extracts the value of a cookie, formatted as per HTTP standard.
//
// This function does not implement the full HTTP standard, just enough to extract
// the cookie value for validation.
func GetCookieValue(line, desired string) *string {
	for len(line) > 0 {
		line = strings.TrimSpace(line)
		if len(line) <= 0 {
			return nil
		}

		var part string
		if split := strings.Index(line, ";"); split > 0 {
			part = strings.TrimSpace(line[:split])
			if len(part) <= 0 {
				return nil
			}

			line = line[split+1:]
		} else {
			part, line = line, ""
		}

		name, val := part, ""
		if split := strings.Index(part, "="); split > 0 {
			name, val = strings.TrimSpace(part[:split]), strings.TrimSpace(part[split+1:])
		}
		if name != desired {
			continue
		}

		if len(val) > 1 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		return &val
	}
	return nil
}

// ExtractCookie returns the value of the first cookie by the specified name.
//
// lines is a list of cookie header lines, as extracted from the HTTP or gRPC
// request.
// name is the name of a cookie.
//
// The value of the cookie is returned, or nil if no cookie by the specified
// name was found.
func ExtractCookie(lines []string, name string) *string {
	if len(lines) <= 0 {
		return nil
	}

	for _, line := range lines {
		value := GetCookieValue(line, name)
		if value != nil {
			return value
		}
	}
	return nil
}

// GetCredentials extracts oauth credentials from a grpc context.
//
// grpc stores cookies and other http headers as metadata available in the
// context. This function extracts and parses an authentication cookie from a
// context.
func GetCredentials(auth *oauth.Extractor, ctx context.Context) (*oauth.CredentialsCookie, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "no cookies in request")
	}
	cookie := ExtractCookie(md["cookie"], auth.CredentialsCookieName())
	if cookie == nil {
		return nil, status.Errorf(codes.Unauthenticated, "no credentials cookie")
	}
	_, creds, err := auth.ParseCredentialsCookie(*cookie)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid credentials - %s", err)
	}
	return creds, nil
}

// ProcessMetadata extracts the grpc metadata from a grpc provided context.Context, and
// verifies that the request was effectively authenticated.
//
// There is no authorization at this layer, just sets the credentials of the user.
func ProcessMetdata(auth *oauth.Extractor, ctx context.Context) (context.Context, error) {
	creds, err := GetCredentials(auth, ctx)
	if err != nil {
		return ctx, err
	}

	return oauth.SetCredentials(ctx, creds), nil
}

// ContextStream is a grpc.ServerStream with a different context attached.
//
// This is necessary as grpc.ServerStream has no mechanism to change the context attached
// with the stream. So, we replace the stream instead.
type ContextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (c *ContextStream) Context() context.Context {
	return c.ctx
}
func SetContextStream(stream grpc.ServerStream, ctx context.Context) *ContextStream {
	if existing, ok := stream.(*ContextStream); ok {
		existing.ctx = ctx
		return existing
	}
	return &ContextStream{ServerStream: stream, ctx: ctx}
}

func StreamInterceptor(auth *oauth.Extractor, unauthenticated ...string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authenticate := true
		for _, direct := range unauthenticated {
			if strings.HasPrefix(info.FullMethod, direct) {
				authenticate = false
			}
		}

		ctx, err := ProcessMetdata(auth, stream.Context())
		if err != nil && authenticate {
			return err
		}
		return handler(srv, SetContextStream(stream, ctx))
	}
}
func UnaryInterceptor(auth *oauth.Extractor, unauthenticated ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authenticate := true
		for _, direct := range unauthenticated {
			if strings.HasPrefix(info.FullMethod, direct) {
				authenticate = false
			}
		}

		ctx, err := ProcessMetdata(auth, ctx)
		if err != nil && authenticate {
			return nil, err
		}
		return handler(ctx, req)
	}
}
