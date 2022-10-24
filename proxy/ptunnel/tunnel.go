package ptunnel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/proxy/nasshp"

	"github.com/gorilla/websocket"
	"github.com/jackpal/gateway"
)

type Tunnel struct {
	log      logger.Logger
	browser  *nasshp.ReplaceableBrowser
	timeouts *Timeouts

	SendWin    *nasshp.BlockingSendWindow
	ReceiveWin *nasshp.BlockingReceiveWindow
}

type GetOptions struct {
	getOptions     []protocol.Modifier
	retryOptions   []retry.Modifier
	connectOptions []ConnectModifier
}

type GetModifier func(*GetOptions) error

type GetModifiers []GetModifier

func (mods GetModifiers) Apply(o *GetOptions) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func WithRetryOptions(mods ...retry.Modifier) GetModifier {
	return func(o *GetOptions) error {
		o.retryOptions = append(o.retryOptions, mods...)
		return nil
	}
}

// Configures options to use in the GET requests to prepare the tunnel.
// This mostly affects the GetSID() call, invoked once per attempt to log in.
func WithGetOptions(mods ...protocol.Modifier) GetModifier {
	return func(o *GetOptions) error {
		o.getOptions = append(o.getOptions, mods...)
		return nil
	}
}

// Configures options to use to establish the websocket moving bytes around.
//
// This mostly affects the Connect(), ConnectSID() and ConnectURL() call,
// invoked once per attempt to establish the websocket used as the actual
// tunnel.
func WithConnectOptions(mods ...ConnectModifier) GetModifier {
	return func(o *GetOptions) error {
		o.connectOptions = append(o.connectOptions, mods...)
		return nil
	}
}

func WithOptions(r *GetOptions) GetModifier {
	return func(o *GetOptions) error {
		*o = *r
		return nil
	}
}

func GetSID(proxy *url.URL, host string, port uint16, mods ...GetModifier) (string, error) {
	curl := *proxy

	params := proxy.Query()
	params.Add("host", host)
	if port > 0 {
		params.Add("port", fmt.Sprintf("%d", port))
	}
	curl.RawQuery = params.Encode()
	curl.Path = path.Join(curl.Path, "/proxy")

	options := &GetOptions{}
	if err := GetModifiers(mods).Apply(options); err != nil {
		return "", err
	}

	retrier := retry.New(options.retryOptions...)

	sid := ""
	err := retrier.Run(func() error {
		err := protocol.Get(curl.String(), protocol.Read(protocol.String(&sid)),
			append([]protocol.Modifier{
				protocol.WithClientOptions(kclient.WithDisabledRedirects()),
				protocol.WithRequestOptions(krequest.AddHeader("Origin", "chrome://enkit-tunnel"))}, options.getOptions...)...)

		herr, ok := err.(*protocol.HTTPError)
		if ok && herr.Resp != nil {
			if herr.Resp.StatusCode == http.StatusTemporaryRedirect {
				return retry.Fatal(kflags.NewIdentityError(
					fmt.Errorf("Proxy %s rejected authentication cookie\n    %w", curl.String(), err),
				))
			}
			if herr.Resp.StatusCode == http.StatusUnauthorized {
				return retry.Fatal(fmt.Errorf("Proxy %s permanently rejected your connection attempt - ACLs?", curl.String()))
			}
		}
		return err
	})
	return sid, err
}

func Connect(proxy *url.URL, host string, port uint16, pos, ack uint32, mods ...GetModifier) (*websocket.Conn, error) {
	options := &GetOptions{}
	if err := GetModifiers(mods).Apply(options); err != nil {
		return nil, err
	}

	sid, err := GetSID(proxy, host, port, WithOptions(options))
	if err != nil {
		return nil, err
	}
	return ConnectSID(proxy, sid, pos, ack, options.connectOptions...)
}

func ConnectSID(proxy *url.URL, sid string, pos, ack uint32, mods ...ConnectModifier) (*websocket.Conn, error) {
	curl := *proxy
	switch {
	case strings.HasPrefix(curl.Scheme, "ws"):
		// Do nothing, the url already has the correct scheme.

	case curl.Scheme == "http":
		curl.Scheme = "ws"
	default:
		curl.Scheme = "wss" // Default to encrypted web sockets.
	}

	params := curl.Query()
	params.Add("sid", strings.TrimSpace(sid))
	params.Add("pos", strconv.FormatUint(uint64(pos), 10))
	params.Add("ack", strconv.FormatUint(uint64(ack), 10))
	curl.RawQuery = params.Encode()
	curl.Path = path.Join(curl.Path, "/connect")

	return ConnectURL(&curl, mods...)
}

type ConnectModifier func(*websocket.Dialer, http.Header) error

type ConnectModifiers []ConnectModifier

func (cm ConnectModifiers) Apply(d *websocket.Dialer, h http.Header) error {
	for _, m := range cm {
		if err := m(d, h); err != nil {
			return err
		}
	}
	return nil
}

func WithHandshakeTimeout(t time.Duration) ConnectModifier {
	return func(d *websocket.Dialer, h http.Header) error {
		d.HandshakeTimeout = t
		return nil
	}
}

func WithBufferSize(read, write int) ConnectModifier {
	return func(d *websocket.Dialer, h http.Header) error {
		d.WriteBufferSize = write
		d.ReadBufferSize = read
		return nil
	}
}

func WithHeader(key, value string) ConnectModifier {
	return func(d *websocket.Dialer, h http.Header) error {
		h.Set(key, value)
		return nil
	}
}

func ConnectURL(curl *url.URL, mods ...ConnectModifier) (*websocket.Conn, error) {
	header := http.Header{}
	header.Add("Origin", "chrome://enkit-tunnel")

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 20 * time.Second
	dialer.WriteBufferSize = 1024 * 16
	dialer.ReadBufferSize = 1024 * 16

	if err := ConnectModifiers(mods).Apply(&dialer, header); err != nil {
		return nil, err
	}

	c, r, err := dialer.Dial(curl.String(), header)
	if err != nil && r != nil {
		// Print the URL without the query parameters, as they contain the SID.
		durl := curl.Scheme + "://" + curl.Hostname() + "/" + curl.EscapedPath()
		switch r.StatusCode {
		case http.StatusTemporaryRedirect:
			return nil, kflags.NewIdentityError(
				fmt.Errorf("Proxy %s rejected authentication cookie\n    %w", durl, err),
			)
		case http.StatusUnauthorized:
			return nil, retry.Fatal(
				fmt.Errorf("Proxy %s permanently rejected the connection - due to ACLs?", durl),
			)
		case http.StatusBadRequest:
			return nil, retry.Fatal(
				fmt.Errorf("Proxy %s permanently rejected the connection - proxy rejects the parameters supplied", durl),
			)
		case http.StatusGone:
			return nil, retry.Fatal(
				fmt.Errorf("Proxy %s permanently rejected the connection - proxy restarted? different region? No longer accepts your session id", durl),
			)
		}
	}
	return c, err
}

type TimeSource func() time.Time

type Timeouts struct {
	Now TimeSource

	ConnWriteTimeout    time.Duration
	BrowserWriteTimeout time.Duration

	BrowserAckInterval  time.Duration
	BrowserPingInterval time.Duration
	BrowserPingTimeout  time.Duration
}

func DefaultTimeouts() *Timeouts {
	to := &Timeouts{
		Now: time.Now,

		ConnWriteTimeout:    time.Second * 20,
		BrowserWriteTimeout: time.Second * 5,

		BrowserAckInterval:  time.Second * 1,
		BrowserPingInterval: time.Second * 10,
	}
	to.BrowserPingTimeout = time.Duration(to.BrowserAckInterval + to.BrowserPingInterval*2)
	return to
}

func (t *Timeouts) Register(set kflags.FlagSet, prefix string) *Timeouts {
	set.DurationVar(&t.ConnWriteTimeout, "conn-write-timeout", t.ConnWriteTimeout,
		"How long to wait for a write to the proxied connection (eg, ssh) to complete before giving up.")
	set.DurationVar(&t.BrowserWriteTimeout, "browser-write-timeout", t.BrowserWriteTimeout,
		"How long to wait for a write to the browser to complete before giving up.")

	set.DurationVar(&t.BrowserAckInterval, "browser-ack-interval", t.BrowserAckInterval, "How long to wait to send pending acks.")
	set.DurationVar(&t.BrowserPingInterval, "browser-ping-interval", t.BrowserPingInterval, "How long to wait before sending a ping.")
	set.DurationVar(&t.BrowserPingTimeout, "browser-ping-timeout", t.BrowserPingTimeout, "How long to wait for a pong to be received before considering the connection dead. Should be greater than browser-ping-interval")

	return t
}

type Flags struct {
	MaxReceiveWindow int
	MaxSendWindow    int

	*Timeouts
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	fl.Timeouts.Register(set, prefix)
	set.IntVar(&fl.MaxSendWindow, prefix+"max-send-window", fl.MaxSendWindow, "Maximum size for the send window")
	set.IntVar(&fl.MaxReceiveWindow, prefix+"max-recv-window", fl.MaxReceiveWindow, "Maximum size for the receive window")
	return fl
}

func DefaultFlags() *Flags {
	return &Flags{
		MaxReceiveWindow: 1048576,
		MaxSendWindow:    1048576,
		Timeouts:         DefaultTimeouts(),
	}
}

type options struct {
	Flags
	Logger logger.Logger
}

func DefaultOptions() *options {
	return &options{
		Flags:  *DefaultFlags(),
		Logger: logger.Nil,
	}
}

type Modifier func(o *options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(o *options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func FromFlags(fl *Flags) Modifier {
	return func(o *options) error {
		// 16 is somewhat arbitrary. Less than 4 bytes would very likely crash.
		// Anything less than 1024 would probably suck in term of performance.
		if fl.MaxSendWindow < 16 {
			return kflags.NewUsageErrorf("invalid max-send-window provided, must be >= 16")
		}
		if fl.MaxReceiveWindow < 16 {
			return kflags.NewUsageErrorf("invalid max-recv-window provided, must be >= 16")
		}

		mods := Modifiers{
			WithWindowSize(fl.MaxSendWindow, fl.MaxReceiveWindow),
			WithTimeouts(fl.Timeouts),
		}
		return mods.Apply(o)
	}
}

func WithTimeouts(t *Timeouts) Modifier {
	return func(o *options) error {
		o.Timeouts = t
		return nil
	}
}

func WithWindowSize(send, receive int) Modifier {
	return func(o *options) error {
		o.MaxSendWindow = send
		o.MaxReceiveWindow = receive
		return nil
	}
}

func WithLogger(l logger.Logger) Modifier {
	return func(o *options) error {
		o.Logger = l
		return nil
	}
}

func NewTunnel(pool *nasshp.BufferPool, mods ...Modifier) (*Tunnel, error) {
	options := DefaultOptions()

	if err := Modifiers(mods).Apply(options); err != nil {
		return nil, err
	}

	tl := &Tunnel{
		log:      options.Logger,
		timeouts: options.Timeouts,

		SendWin:    nasshp.NewBlockingSendWindow(pool, uint64(options.MaxSendWindow)),
		ReceiveWin: nasshp.NewBlockingReceiveWindow(pool, uint64(options.MaxReceiveWindow)),
		browser:    nasshp.NewReplaceableBrowser(options.Logger, nil)}

	go tl.BrowserReceive()
	go tl.BrowserSend()

	return tl, nil
}

// CloseRequested error is returned to interrupt Reads/Writes once a Close()
// has been requested by the user.
var CloseRequested = errors.New("close requested")

func (t *Tunnel) Close() {
	err := CloseRequested
	t.browser.Close(err)
	t.SendWin.Fail(err)
	t.ReceiveWin.Fail(err)
}

func (t *Tunnel) KeepConnected(proxy *url.URL, host string, port uint16, mods ...GetModifier) error {
	options := &GetOptions{
		retryOptions: []retry.Modifier{retry.WithAttempts(0), retry.WithLogger(t.log), retry.WithDescription(fmt.Sprintf("connecting to %s:%d via %s", host, port, proxy.String()))},
	}
	if err := GetModifiers(mods).Apply(options); err != nil {
		return err
	}

	sid, err := GetSID(proxy, host, port, WithOptions(options))
	if err != nil {
		return err
	}

	retrier := retry.New(options.retryOptions...)
	return retrier.Run(func() error {
		// Following the nassh documentation, at:
		//  https://chromium.googlesource.com/apps/libapps/+/4763ff7fa95760c9c85ef3563953cdfb391d209f/nassh/doc/relay-protocol.md
		// pos: "... the last write ack the client received" -> WrittenUntil
		// ack: "... the last read ack the client received" -> ReadUntil

		pos, ack := t.browser.GetWriteReadUntil()
		conn, err := ConnectSID(proxy, sid, pos, ack, options.connectOptions...)
		if err != nil {
			return err
		}

		conn.SetReadDeadline(t.timeouts.Now().Add(t.timeouts.BrowserPingTimeout))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(t.timeouts.Now().Add(t.timeouts.BrowserPingTimeout))
			return nil
		})

		waiter := t.browser.Set(conn, ack, pos)
		if err := waiter.Wait(); !errors.Is(err, CloseRequested) {
			return err
		}
		return nil
	})
}

func (t *Tunnel) BrowserSend() error {
	ackbuffer := [4]byte{}

	var nextping time.Time
	var conn *websocket.Conn
	var oldru uint32

outer:
	for {
		if err := t.SendWin.WaitToEmpty(t.timeouts.BrowserAckInterval); err != nil && err != nasshp.ErrorExpired {
			t.browser.Close(fmt.Errorf("stopping browser write - reader returned %s", err))
			return err
		}

		nconn, wu, ru, err := t.browser.GetForSend()
		if err != nil {
			t.browser.Close(fmt.Errorf("stopping browser write: %w", err))
			return err
		}

		if nconn != conn {
			if err := t.SendWin.Reset(wu); err != nil {
				t.browser.Close(fmt.Errorf("stopping browser write after failed reset: %w", err))
				return err
			}
			conn = nconn
		} else {
			if acked, err := t.SendWin.AcknowledgeUntil(wu); err != nil {
				t.browser.PushWrittenUntil((uint32)(acked & 0xffffff))
				// TODO: fix the recovery problem on the server side.
				//t.browser.Close(fmt.Errorf("stopping browser write after failed acknowledge: %w", err))
				//return err
			}
		}

		now := t.timeouts.Now()
		conn.SetWriteDeadline(now.Add(t.timeouts.BrowserWriteTimeout))
		if now.After(nextping) {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				t.browser.Error(conn, fmt.Errorf("websocket Ping returned error: %w", err))
			}
			nextping = now.Add(t.timeouts.BrowserPingInterval)
		}

		// We may be here because there's either data to send, or we need to ack data back to the browser.
		buffer := t.SendWin.ToEmpty()
		if len(buffer) == 0 && ru == oldru {
			continue
		}
		oldru = ru

		writer, err := conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("websocket NextWriter returned error: %w", err))
			continue
		}

		binary.BigEndian.PutUint32(ackbuffer[:], ru)
		written, err := writer.Write(ackbuffer[:])
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("browser ack write failed with %w", err))
			continue
		}
		if written != 4 {
			t.browser.Error(conn, fmt.Errorf("browser ack write resulted in less than 4 bytes written"))
			continue
		}

		for {
			written, err = writer.Write(buffer)
			if err != nil {
				t.browser.Error(conn, fmt.Errorf("browser data write resulted in error %w", err))
				continue outer
			}
			t.SendWin.Empty(written)

			buffer = t.SendWin.ToEmpty()
			if len(buffer) == 0 {
				break
			}
		}

		if err := writer.Close(); err != nil {
			t.browser.Error(conn, fmt.Errorf("browser data flush resulted in error %w", err))
			continue
		}
	}
}

func (t *Tunnel) BrowserReceive() error {
	ackbuffer := [4]byte{}
	var conn *websocket.Conn

retry:
	for {
		if err := t.ReceiveWin.WaitToFill(); err != nil {
			t.browser.Close(fmt.Errorf("stopping browser read - writer returned: %w", err))
			return err
		}

		nconn, ru, err := t.browser.GetForReceive()
		if err != nil {
			t.browser.Close(fmt.Errorf("stopping browser read: %w", err))
			return err
		}

		if nconn != conn {
			if err := t.ReceiveWin.Reset(ru); err != nil {
				t.browser.Close(fmt.Errorf("browser receive reset failed: %w", err))
				continue
			}
			conn = nconn
		}

		_, r, err := conn.NextReader()
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("websocket NextReader returned error: %w", err))
			continue
		}

		// Retry the read until the ack (4 bytes, unit32) has been read fully.
		for ackread := 0; ackread < len(ackbuffer); {
			size, err := r.Read(ackbuffer[ackread:])
			if err != nil {
				if err != io.EOF {
					t.browser.Error(conn, err)
				}
				continue retry
			}
			ackread += size
		}

		ack := binary.BigEndian.Uint32(ackbuffer[:])
		if ack&0xff000000 != 0 {
			t.browser.Error(conn, fmt.Errorf("browser read resulted in ack requesting connection reset (%08x)", ack))
			continue
		}
		t.browser.PushWrittenUntil(ack)

		for {
			buffer := t.ReceiveWin.ToFill()
			if len(buffer) == 0 {
				break
			}

			size, err := r.Read(buffer)
			if err != nil {
				if err != io.EOF {
					t.browser.Error(conn, fmt.Errorf("browser read failed with %w", err))
				}
				break
			}
			conn.SetReadDeadline(t.timeouts.Now().Add(t.timeouts.BrowserPingTimeout))
			filled := t.ReceiveWin.Fill(size)
			t.browser.PushReadUntil((uint32)(filled) & 0xffffff)
		}
	}
}

func (t *Tunnel) Send(file io.Reader) error {
	for {
		if err := t.SendWin.WaitToFill(); err != nil {
			return err
		}

		buffer := t.SendWin.ToFill()
		size, err := file.Read(buffer)
		if err != nil {
			return err
		}
		t.SendWin.Fill(size)
	}
}

type Flushable interface {
	Flush() error
}

type WithWriteDeadline interface {
	SetWriteDeadline(time.Time) error
}

func (t *Tunnel) Receive(file io.Writer) error {
	flushable, canflush := file.(Flushable)
	timeouttable, cantimeout := file.(WithWriteDeadline)

	for {
		if err := t.ReceiveWin.WaitToEmpty(); err != nil {
			return err
		}

		for {
			buffer := t.ReceiveWin.ToEmpty()
			if len(buffer) == 0 {
				break
			}
			if cantimeout {
				if err := timeouttable.SetWriteDeadline(t.timeouts.Now().Add(t.timeouts.ConnWriteTimeout)); err != nil {
					if !errors.Is(err, os.ErrNoDeadline) {
						return err
					}
					cantimeout = false
				}
			}
			size, err := file.Write(buffer)
			if err != nil {
				return err
			}

			t.ReceiveWin.Empty(size)
			if canflush {
				if err := flushable.Flush(); err != nil {
					return err
				}
			}

		}
	}
}

type TunnelType int

const (
	TunnelTypeNone TunnelType = iota
	TunnelTypePersistent
	TunnelTypeLocal
)

func TunnelTypeForHost(host string) (TunnelType, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return TunnelTypeNone, fmt.Errorf("failed to look up %q: %w", host, err)
	}
	if len(ips) < 1 {
		return TunnelTypeNone, fmt.Errorf("expected at least one IP for host %q; got 0", host)
	}
	// Assumption: only the first IP in the returned record is used. We
	// currently don't have any instances of multiple IPs for our services.
	ip := ips[0]

	// If the IP resolves to the local system, the expectation is that a tunnel
	// is created to reach the host.
	if ip.IsLoopback() {
		return TunnelTypeLocal, nil
	}
	// IP is not loopback, so it is either the gateway, or the IP of the actual
	// target.

	gw, err := gateway.DiscoverGateway()
	if err != nil {
		return TunnelTypeNone, fmt.Errorf("failed to discover gateway: %w", err)
	}

	// If the IP resolves to the gateway, the connection is assumed to be going
	// through a persistent tunnel.
	if ip.Equal(gw) {
		return TunnelTypePersistent, nil
	}

	// The IP is assumed to be that of the actual target; no tunnel needed.
	return TunnelTypeNone, nil
}
