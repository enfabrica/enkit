package client

import (
	"github.com/enfabrica/enkit/lib/grpcwebclient"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"

	"context"
	"fmt"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
)

type ServerFlags struct {
	Prefix, Name  string
	Server        string
	AllowInsecure bool
}

func (sf *ServerFlags) Register(store *pflag.FlagSet, prefix, name, path string) {
	store.BoolVar(&sf.AllowInsecure, prefix+"-allow-insecure", false, "Allow insecure connections (disable https checks, gRPC security) to the "+name)
	store.StringVar(&sf.Server, prefix+"-server", path, name+" to connect to. If the URL starts with http:// or https://, it will use the grpc-web protocol")

	// TODO: provide a reasonable constructor, so we don't have to do this.
	sf.Prefix = prefix
	sf.Name = name
}

func (sf *ServerFlags) Connect(mods ...GwcOrGrpcOptions) (grpc.ClientConnInterface, error) {
	if sf.Server == "" {
		if sf.Prefix != "" && sf.Name != "" {
			return nil, kflags.NewUsageError(fmt.Errorf("Must specify the address for %s, use --%s", sf.Name, sf.Prefix+"-server"))
		}
		return nil, fmt.Errorf("Must specify the address of the server")
	}

	if sf.AllowInsecure {
		mods = append(mods, WithInsecure())
	}

	return Connect(sf.Server, mods...)
}

type GwcOrGrpcOption interface{}
type GwcOrGrpcOptions []GwcOrGrpcOption

func WithInsecure() GwcOrGrpcOptions {
	return GwcOrGrpcOptions{gwc.WithHttpSettings(kclient.WithInsecureCertificates()), grpc.WithInsecure()}
}

func SetCookieUnaryInterceptor(cookie *http.Cookie) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "cookie", cookie.String())
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func SetCookieStreamInterceptor(cookie *http.Cookie) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, "cookie", cookie.String())
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func WithCookie(cookie *http.Cookie) GwcOrGrpcOptions {
	return GwcOrGrpcOptions{
		gwc.WithRequestSettings(krequest.WithCookie(cookie)),
		grpc.WithUnaryInterceptor(SetCookieUnaryInterceptor(cookie)),
		grpc.WithStreamInterceptor(SetCookieStreamInterceptor(cookie)),
	}
}

func Connect(server string, mods ...GwcOrGrpcOptions) (grpc.ClientConnInterface, error) {
	grpcOptions := []grpc.DialOption{}
	gwcOptions := []gwc.Modifier{}
	for _, m := range mods {
		for _, o := range m {
			switch t := o.(type) {
			case gwc.Modifier:
				gwcOptions = append(gwcOptions, t)
			case grpc.DialOption:
				grpcOptions = append(grpcOptions, t)
			default:
				return nil, fmt.Errorf("API Usage Error - option can be a gwc.Modifier or a grpc.DialOption - %#v unknown", o)
			}
		}
	}

	if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
		return gwc.New(server, gwcOptions...)
	}
	return grpc.Dial(server, grpcOptions...)
}

// NiceError turns a grpc error into a more user friendly message.
func NiceError(err error, formatting string, args ...interface{}) error {
	switch status.Code(err) {
	case codes.Unauthenticated:
		return fmt.Errorf("Authentication token expired? Do you need to log in again? %s", err)
	default:
		return fmt.Errorf(formatting, args...)
	}
}
