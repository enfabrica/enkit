package client

import (
	"github.com/enfabrica/enkit/lib/grpcwebclient"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"

	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
)

type ServerFlags struct {
	Name, Description string
	Server            string
	AllowInsecure     bool
}

// DefaultServerFlags initializes and returns a ServerFlags object with defaults.
//
// name is a string to prepend to the registered flags. For example, an "auth" prefix will result
// in the flags --auth-server and --auth-allow-insecure to be registerd. This allows to have
// multiple servers defined by flags in the same binary.
//
// description is a description for the server, which will be used to create the help messages. For example,
// a name of "Authentication server" will be appended to "allow... connections to the... Authentication server".
//
// address is the default address of the server to connect to, leave it empty if unknown.
func DefaultServerFlags(name, description, address string) *ServerFlags {
	return &ServerFlags{
		Name:        name,
		Description: description,
		Server:      address,
	}
}

func (sf *ServerFlags) Register(store kflags.FlagSet, namespace string) {
	store.BoolVar(&sf.AllowInsecure, namespace+sf.Name+"-allow-insecure", sf.AllowInsecure, "Allow insecure connections (disable https checks, gRPC security) to the "+sf.Description)
	store.StringVar(&sf.Server, namespace+sf.Name+"-server", sf.Server, sf.Description+" to connect to. If the URL starts with http:// or https://, it will use the grpc-web protocol")
}

func (sf *ServerFlags) Connect(mods ...GwcOrGrpcOptions) (grpc.ClientConnInterface, error) {
	if sf.Server == "" {
		return nil, kflags.NewUsageErrorf("Must specify the address for %s, use --%s-server", sf.Description, sf.Name)
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
		return kflags.NewIdentityError(err)
	default:
		return fmt.Errorf(formatting, args...)
	}
}
