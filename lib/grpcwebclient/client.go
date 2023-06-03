package gwc

import (
	"context"
	"net/http"
	"net/http/httputil"

	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

type Modifier func(c *Client) error

type Client struct {
	Url string
	// Set to true if an alert was generated.
	alarmed bool

	MaxReceiveSize  int64
	MaxSendSize     int64
	HttpModifier    kclient.Modifiers
	RequestModifier krequest.Modifiers
}

func WithMaxReceiveSize(size int64) Modifier {
	return func(c *Client) error {
		c.MaxReceiveSize = size
		return nil
	}
}

func WithMaxSendSize(size int64) Modifier {
	return func(c *Client) error {
		c.MaxSendSize = size
		return nil
	}
}

func WithDisabledWarning() Modifier {
	return func(c *Client) error {
		c.alarmed = true
		return nil
	}
}

func WithHttpSettings(mods ...kclient.Modifier) Modifier {
	return func(c *Client) error {
		c.HttpModifier = append(c.HttpModifier, mods...)
		return nil
	}
}

func WithRequestSettings(mods ...krequest.Modifier) Modifier {
	return func(c *Client) error {
		c.RequestModifier = append(c.RequestModifier, mods...)
		return nil
	}
}

func New(url string, mods ...Modifier) (*Client, error) {
	c := &Client{Url: url, MaxReceiveSize: 16 * 1048576, MaxSendSize: 16 * 1048576}
	for _, mod := range mods {
		if err := mod(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func Marshal(data interface{}) ([]byte, error) {
	if marshable, ok := data.(proto.Marshaler); ok {
		return marshable.Marshal()
	}

	message, ok := data.(proto.Message)
	if !ok {
		return nil, status.Errorf(codes.Internal, "grpc-web-client can only marshal protocol buffers - got %#v", data)
	}
	return proto.Marshal(message)
}

func Unmarshal(data []byte, msg interface{}) error {
	if unmarshable, ok := msg.(proto.Unmarshaler); ok {
		return unmarshable.Unmarshal(data)
	}

	message, ok := msg.(proto.Message)
	if !ok {
		return status.Errorf(codes.Internal, "grpc-web-client can only unmarshal protocol buffers - got %#v", msg)
	}

	return proto.Unmarshal(data, message)
}

func CheckLimit(value int64, limit *int64) error {
	if *limit == 0 {
		return nil
	}

	if value > *limit {
		return fmt.Errorf("too many bytes in request - tune with MaxReceiveSize - exceeds limit by %d", value-*limit)
	}
	*limit -= value
	return nil
}

func ReadChunk(r io.Reader, limit *int64) (byte, []byte, error) {
	hdr := [5]byte{}
	if err := CheckLimit(int64(len(hdr)), limit); err != nil {
		return '\xff', nil, err
	}

	// This is the only read where receiving EOF is ok. In any other read, it means the message
	// was truncated one way or another.
	if got, err := io.ReadFull(r, hdr[:]); err != nil {
		if err == io.EOF && got == 0 {
			return '\xff', nil, err
		}
		return '\xff', nil, fmt.Errorf("packing header not found - %w", err)
	}
	msglen := binary.BigEndian.Uint32(hdr[1:])
	if err := CheckLimit(int64(msglen), limit); err != nil {
		return hdr[0], nil, err
	}

	msgdata := make([]byte, msglen)
	if got, err := io.ReadFull(r, msgdata); err != nil && (err != io.EOF || got != len(msgdata)) {
		return hdr[0], msgdata[:got], err
	}
	return hdr[0], msgdata, nil
}

func ReadMessage(r io.Reader, reply interface{}, limit *int64) error {
	flags, msgdata, err := ReadChunk(r, limit)
	if err != nil {
		return err
	}

	if flags != 0 {
		return fmt.Errorf("invalid first byte of response - 0x00 expected, got %02x", flags)
	}
	if err := Unmarshal(msgdata, reply); err != nil {
		if status.Code(err) == codes.Unknown {
			return status.Errorf(codes.Internal, "message was not parsed correctly: %x - %v", msgdata, err)
		}
		return err
	}
	return nil
}

func ReadTrailer(r io.Reader, limit *int64) (string, string, error) {
	flags, msgdata, err := ReadChunk(r, limit)
	if err != nil {
		return "", "", err
	}

	if flags != 0x80 {
		return "", "", fmt.Errorf("invalid first byte - 0x80 expected, got %02x", flags)
	}

	key := string(msgdata)
	idx := strings.Index(key, ": ")
	if idx < 0 {
		return key, "", nil
	}
	return key[:idx], key[idx+2:], nil
}

func (c *Client) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if len(opts) > 0 && !c.alarmed {
		c.alarmed = true
		log.Printf("WARNING: grpc-web-client is being used with unsupported options - ask your favorite golang dev to fix it - options: %#v", opts)
	}
	// Request format has:
	//   1 byte  - bitfield, indicating compression and serialization.
	//   4 bytes - length of the protocol message.
	//   [...]   - serialized protocol message.
	data, err := Marshal(args)
	if err != nil {
		return err
	}

	hdr := [5]byte{}
	hdr[0] = 0
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(data)))

	if c.MaxSendSize > 0 && int64(len(data)+len(hdr)) > c.MaxSendSize {
		return status.Errorf(codes.Internal, "request exceeds maximum size of %d - %d (use MaxSendSize to increase)", c.MaxSendSize, len(data)+len(hdr))
	}

	client := &http.Client{}
	if err := c.HttpModifier.Apply(client); err != nil {
		return status.Errorf(codes.Internal, "could not initialize http client %s", err)
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Url+method, io.MultiReader(bytes.NewReader(hdr[:]), bytes.NewReader(data)))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create an http request - %s", err)
	}

	req.Header.Set("Content-Type", "application/grpc-web+proto")
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("X-User-Agent", "grpc-web-go/0.0")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)+len(hdr)))

	if err := c.RequestModifier.Apply(req); err != nil {
		return status.Errorf(codes.Internal, "could not initialize request %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Response format has:
	//   a bunch of things in headers (Grpc-Message, Grpc-Status the main ones).
	//   1 byte  - bitfield, indicating what is coming. Either a message (0x00) or a trailer (0x80).
	//   4 bytes - length of the protocol message.
	//   [...]   - serialized protocol message.
	if ct := resp.Header.Get("Content-Type"); ct != "application/grpc-web+proto" {
		content, _ := httputil.DumpResponse(resp, true)
		return status.Errorf(codes.Internal, "unknown content-type in response - only application/grpc-web+proto "+
			"is supported - full response dump:\n------\n%s\n------", string(content))
	}
	message := resp.Header.Get("Grpc-Message")
	strcode := resp.Header.Get("Grpc-Status")
	if strcode != "0" && strcode != "" {
		intcode, err := strconv.Atoi(strcode)
		if err != nil {
			return status.Errorf(codes.Internal, "RPC failed - but could not parse error - %s (%s)", strcode, message)
		}
		return status.Error(codes.Code(intcode), message)
	}

	limit := c.MaxReceiveSize
	if err := ReadMessage(resp.Body, reply, &limit); err != nil {
		return err
	}

	// This loop just discards trailers.
	for {
		_, _, err := ReadTrailer(resp.Body, &limit)
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Errorf(codes.Internal, "unexpected error parsing trailers - %s", err)
		}
	}
	return nil
}

func (c *Client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Errorf(codes.Unimplemented, "grpc-web-client only implements unary requests")
}
