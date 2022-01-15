package nasshp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/gorilla/websocket"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type NasshProxy struct {
	log           logger.Logger
	upgrader      websocket.Upgrader
	authenticator oauth.Authenticate
	encoder       token.BinaryEncoder
	filter        Filter

	// Timeouts to use, must be set.
	timeouts *Timeouts

	// Where to redirect users after authentication to get their connections going.
	relayHost string

	// sync.Pool of buffers to allocate and use for clients.
	pool *BufferPool

	// Key is a string, the SID. Value is a readWriter pointer.
	connections sync.Map
}

func (np *NasshProxy) RelayHost() string {
	return np.relayHost
}

type options struct {
	bufferSize       int
	symmetricSetters []token.SymmetricSetter
	rng              *rand.Rand
}

type Flags struct {
	*Timeouts

	SymmetricKey []byte
	BufferSize   int
	RelayHost    string
}

func DefaultTimeouts() *Timeouts {
	return &Timeouts{
		Now: time.Now,

		BrowserWriteTimeout: time.Second * 60,
		BrowserAckTimeout:   time.Second * 1,
		ConnWriteTimeout:    time.Second * 60,
		ResolutionTimeout:   time.Second * 30,

		// TODO: add a maximum idle time.
	}
}

func DefaultFlags() *Flags {
	return &Flags{
		Timeouts: DefaultTimeouts(),
	}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.ByteFileVar(&fl.SymmetricKey, prefix+"sid-encryption-key", "",
		"Path of the file containing the symmetric key to use to create/process sids. "+
			"If not supplied, a new key is generated")
	set.IntVar(&fl.BufferSize, prefix+"buffer-size", 8192, "Size of the buffers to use to send/receive data for connections")
	set.StringVar(&fl.RelayHost, prefix+"host-port", "", "The hostname and port number the nassh client has to be redirected to to establish an ssh connection. "+
		"Typically, this is the DNS name and port 80 or 443 of the host running this proxy.")

	set.DurationVar(&fl.BrowserWriteTimeout, "browser-write-timeout", fl.BrowserWriteTimeout,
		"How long to wait for a write to the browser to complete before giving up.")
	set.DurationVar(&fl.BrowserAckTimeout, "browser-ack-timeout", fl.BrowserAckTimeout,
		"How long to wait before sending an ack back.")
	set.DurationVar(&fl.ConnWriteTimeout, "conn-write-timeout", fl.ConnWriteTimeout,
		"How long to wait for a write to the proxied connection (eg, ssh) to complete before giving up.")
	set.DurationVar(&fl.ResolutionTimeout, "resolution-timeout", fl.ResolutionTimeout,
		"How long to wait to resolve the name of the destination of the proxied connection")
	return fl
}

type Modifier func(*NasshProxy, *options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(np *NasshProxy, o *options) error {
	for _, m := range mods {
		if err := m(np, o); err != nil {
			return err
		}
	}
	return nil
}

func WithLogging(log logger.Logger) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.log = log
		return nil
	}
}

func WithBufferSize(size int) Modifier {
	return func(np *NasshProxy, o *options) error {
		o.bufferSize = size
		return nil
	}
}

func WithRelayHost(relayHost string) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.relayHost = relayHost
		return nil
	}
}

type Filter func(proto string, hostport string, creds *oauth.CredentialsCookie) Verdict

func WithFilter(filter Filter) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.filter = filter
		return nil
	}
}

func FromFlags(fl *Flags) Modifier {
	return func(np *NasshProxy, o *options) error {
		relayHost := strings.TrimSpace(fl.RelayHost)
		if len(relayHost) == 0 {
			return kflags.NewUsageErrorf("specifying --host-port is mandatory - there's no automated way to guess a dns name and port where this proxy (or a relay) is running")
		}

		if len(fl.SymmetricKey) == 0 {
			key, err := token.GenerateSymmetricKey(o.rng, 0)
			if err != nil {
				return fmt.Errorf("the world is about to end, even random number generators are failing - %w", err)
			}
			fl.SymmetricKey = key
		}
		mods := Modifiers{
			WithRelayHost(fl.RelayHost),
			WithSymmetricOptions(token.UseSymmetricKey(fl.SymmetricKey)),
			WithBufferSize(fl.BufferSize),
			WithTimeouts(fl.Timeouts),
		}
		return mods.Apply(np, o)
	}
}

func WithSymmetricOptions(mods ...token.SymmetricSetter) Modifier {
	return func(np *NasshProxy, o *options) error {
		o.symmetricSetters = append(o.symmetricSetters, mods...)
		return nil
	}
}

func WithOriginChecker(checker func(r *http.Request) bool) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.upgrader.CheckOrigin = checker
		return nil
	}
}

func WithTimeouts(timeouts *Timeouts) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.timeouts = timeouts
		return nil
	}
}

// New creates a new instance of a nasshp tunnel protocol.
//
// rng MUST be a secure random number generator, use github.com/enfabrica/lib/srand
// in case of doubt to create one.
// authenticator is optional, can be left to nil to disable authentication.
//
// mods MUST either contain FromFlags, to initialize all the nassh parameters from
// command line flags, or it MUST provide a symmetric key with nasshp.WithSymmetricOptions.
func New(rng *rand.Rand, authenticator oauth.Authenticate, mods ...Modifier) (*NasshProxy, error) {
	o := &options{rng: rng, bufferSize: 8192}
	np := &NasshProxy{
		timeouts:      DefaultTimeouts(),
		authenticator: authenticator,
		log:           logger.Nil,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					return false
				}
				return strings.HasPrefix(origin, "chrome-extension://") || strings.HasPrefix(origin, "chrome://")
			},
		},
	}

	if err := Modifiers(mods).Apply(np, o); err != nil {
		return nil, err
	}

	if np.encoder == nil {
		be, err := token.NewSymmetricEncoder(rng, o.symmetricSetters...)
		if err != nil {
			return nil, fmt.Errorf("nassh - error setting up symmetric encryption: %w", err)
		}

		ue := token.NewBase64UrlEncoder()

		np.encoder = token.NewChainedEncoder(be, ue)
	}
	if np.pool == nil {
		np.pool = NewBufferPool(o.bufferSize)
	}
	return np, nil
}

type MuxHandle func(pattern string, handler func(http.ResponseWriter, *http.Request))

func (np *NasshProxy) ServeCookie(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	ext := params.Get("ext")
	path := params.Get("path")
	if ext == "" || path == "" {
		http.Error(w, fmt.Sprintf("invalid request for: %s", r.URL), http.StatusBadRequest)
		return
	}

	target := &url.URL{
		Scheme:   "chrome-extension",
		Path:     ext + "/" + path,
		Fragment: "nasshp-enkit@" + np.relayHost,
	}

	if np.authenticator != nil {
		creds, err := np.authenticator(w, r, target)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid request for: %s - %s", r.URL, err), http.StatusBadRequest)
			return
		}
		// There are no credentials, so the user has been redirected to the authentication service.
		if creds == nil {
			return
		}
	}
	http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
}

var OriginMatcher = regexp.MustCompile(`^chrome(-extension)?://`)

func (np *NasshProxy) ServeProxy(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	host := params.Get("host")
	port := params.Get("port")
	origin := r.Header.Get("Origin")
	if origin != "" && OriginMatcher.MatchString(origin) {
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}
	if np.authenticator != nil {
		creds, err := np.authenticator(w, r, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid request for: %s - %s", r.URL, err), http.StatusBadRequest)
			return
		}
		if creds == nil {
			return
		}
	}

	sp, err := strconv.ParseUint(port, 10, 16)
	if err != nil || port == "" {
		http.Error(w, fmt.Sprintf("invalid port requested: %s", port), http.StatusBadRequest)
		return
	}
	if host == "" {
		http.Error(w, fmt.Sprintf("invalid empty host: %s", host), http.StatusBadRequest)
		return
	}
	hostport := net.JoinHostPort(host, strconv.Itoa(int(sp)))

	_, allowed := np.allow(r, w, "", hostport)
	if !allowed {
		return
	}

	sid, err := np.encoder.Encode([]byte(hostport))
	if err != nil {
		http.Error(w, "Sorry, the world is coming to an end, there was an error generating a session id. Good Luck.", http.StatusInternalServerError)
	}
	fmt.Fprintln(w, string(sid))
}

func LogId(sid string, r *http.Request, hostport string, c *oauth.CredentialsCookie) string {
	identity := ""
	if c != nil {
		identity = "[" + c.Identity.GlobalName() + "]"
	}
	if sid != "" {
		sid = "[SID:" + sid[0:6] + "]"
	}
	return fmt.Sprintf("%s[IP:%s][DEST:%s]%s", sid, r.RemoteAddr, hostport, identity)
}

func (np *NasshProxy) allow(r *http.Request, w http.ResponseWriter, sid, hostport string) (string, bool) {
	logid := LogId(sid, r, hostport, nil)

	var creds *oauth.CredentialsCookie
	if np.authenticator != nil {
		// TODO: merge credentials checking with filtering mechanism.
		creds, err := np.authenticator(w, r, nil)
		if err != nil {
			np.log.Warnf("%s - authentication error: %s", logid, err)
			http.Error(w, fmt.Sprintf("invalid request for: %s - %s", r.URL, err), http.StatusBadRequest)
			return logid, false
		}
		if creds == nil {
			return logid, false
		}
		logid = LogId(sid, r, hostport, creds)
	}
	if np.filter != nil {
		host, port, err := net.SplitHostPort(hostport)
		if err != nil {
			np.log.Infof("%s - err %v splitting host and port %s", logid, err, hostport)
			http.Error(w, fmt.Sprintf("Go somewhere else, you are not allowed to connect here."), http.StatusUnauthorized)
			return logid, false
		}
		res, err := net.LookupHost(host)
		if err != nil {
			np.log.Infof("%s - err %v looking up host %s", logid, err, host)
			http.Error(w, fmt.Sprintf("Go somewhere else, you are not allowed to connect here."), http.StatusUnauthorized)
			return logid, false
		}
		verdict := VerdictUnknown
		for _, u := range res {
			// TODO(adam): make verdict merging configurable from ACL list
			// TODO(adam): return here after making authz engine
			verdict = verdict.MergeOnlyAcceptAllow(np.filter("tcp", net.JoinHostPort(u, port), creds))
		}
		if verdict == VerdictAllow {
			return logid, true
		}
		np.log.Infof("%s was rejected by filter", logid)
		http.Error(w, fmt.Sprintf("Go somewhere else, you are not allowed to connect here."), http.StatusUnauthorized)
		return logid, false
	}
	return logid, true
}

func (np *NasshProxy) ServeConnect(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	sid := params.Get("sid")
	ack := strings.TrimSpace(params.Get("ack"))
	pos := strings.TrimSpace(params.Get("pos"))

	_, hostportb, err := np.encoder.Decode(context.Background(), []byte(sid))
	if err != nil {
		http.Error(w, "invalid sid provided", http.StatusBadRequest)
		return
	}
	hostport := string(hostportb)

	logid := LogId(sid, r, hostport, nil)

	logid, allow := np.allow(r, w, sid, hostport)
	if !allow {
		return
	}
	np.log.Infof("%s - connect allowed", logid)

	rack := uint32(0)
	if ack != "" {
		sp, err := strconv.ParseUint(ack, 10, 32)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid ack requested: %s - %s", ack, err), http.StatusBadRequest)
			return
		}
		rack = uint32(sp)
	}

	wack := uint32(0)
	if pos != "" {
		sp, err := strconv.ParseUint(pos, 10, 32)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid pos requested: %s - %s", pos, err), http.StatusBadRequest)
			return
		}
		wack = uint32(sp)
	}

	err = np.ProxySsh(logid, r, w, sid, rack, wack, string(hostportb))
	if err != nil {
		if err != io.EOF {
			np.log.Warnf("%s connection with %v dropped: %v", logid, r.RemoteAddr, err)
		}
		return
	}
}

type TimeSource func() time.Time

// readWriter holds the state to read from an ssh connection and writes to the browser.
//
// There are a few nuisances here:
// - once something is written into (or read from) the ssh connection, it cannot be taken back.
// - but the browser can appear or disappear at any time, there can even be old lingering sessions pending.
//
// The approach used:
// - when reading from ssh and writing into the browser, we don't get rid of the buffer with the data
//   until the browser acknowledges reception.
//   If there is no browser active at that time, the code will keep waiting for a browser to become
//   available without consuming the ssh buffer, which should cause back pressure on the terminal.
// - when reading from the browser and writing into ssh, we discard any data up to the point we have
//   already written. If there is a gap, there's nothing we can really do, as the protocol does not
//   provide any mechanism to ask for missing data.
type readWriter struct {
	*Timeouts
	pool *BufferPool

	// Browser{Read,Write}Ack are 32 bits unsigned.
	//
	// The NASSH protocol defines them as made of 24 bits absolute counter of data read/written,
	// with the first 8 bits being reserved, but indicating an error if set.
	// Use sync.atomic Load / Store operations to access them.
	browserReadAck  uint32
	browserWriteAck uint32

	// pending{Read,Write}Ack are what the browser just asked after a re-connection.
	// They can be invalid, they cannot be trusted. If 0, no browser request was performed.
	// Use synca.atomic Load / Store operations to access them.
	pendingReadAck  uint32
	pendingWriteAck uint32

	browser ReplaceableBrowser
	conn    net.Conn

	waiter waiter
}

func newReadWriter(log logger.Logger, pool *BufferPool, timeouts *Timeouts) *readWriter {
	rw := &readWriter{Timeouts: timeouts, pool: pool}
	rw.browser.Init(log)
	return rw
}

type Timeouts struct {
	Now TimeSource

	ResolutionTimeout time.Duration

	BrowserWriteTimeout time.Duration
	BrowserAckTimeout   time.Duration

	ConnWriteTimeout time.Duration
}

// readFromBrowser reads from the browser, and writes to the ssh connection.
//
// Problems:
// - the only way to stop a read that is in progress is to close the browser connection.
// - we may need to read data and well, discard it.
func (np *readWriter) proxyFromBrowser(ssh net.Conn) (err error) {
	defer func() {
		if err != nil {
			np.browser.log.Warnf("browser error %s", err)
			return
		}
	}()

	ackbuffer := [4]byte{}

	wrecv := NewReceiveWindow(np.pool)

	defer ssh.Close()
	defer wrecv.Drop()

	for {
		// The browser WACK is the server RACK.
		wc, rack, wack, err := np.browser.GetRack()
		if err != nil {
			return err
		}
		if rack != 0 {
			atomic.StoreUint32(&np.browserReadAck, uint32(rack)&0xffffff)
			if err := wrecv.Reset(rack); err != nil {
				err := fmt.Errorf("browser ack reset failed: %w", err)
				np.browser.Close(err)
				return err
			}
		}

		_, browser, err := wc.NextReader()
		if err != nil {
			np.browser.Error(wc, err)
			continue
		}

		read, err := browser.Read(ackbuffer[:])
		if err != nil {
			if err != io.EOF {
				np.browser.Error(wc, err)
			}
			continue
		}

		// Short reads would be possible in a normal socket, but here we are reading a framed websocket
		// message from an http connection, consuming data from a pre-existing buffer.
		if read < 4 {
			np.browser.Error(wc, fmt.Errorf("short read from browser: %d bytes only", read))
			continue
		}
		wack = binary.BigEndian.Uint32(ackbuffer[:])
		if wack&0xff000000 != 0 {
			np.browser.Error(wc, fmt.Errorf("browser told us there was an error by setting bits %01x", (wack&0xff000000)>>24))
			continue
		}
		atomic.StoreUint32(&np.browserWriteAck, wack)

		for {
			buffer := wrecv.ToFill()
			read, err := browser.Read(buffer)
			if err != nil {
				if err != io.EOF {
					np.browser.Error(wc, err)
				}
				break
			}
			wrecv.Fill(read)
			buffer = buffer[:read]

			for {
				toSend := wrecv.ToEmpty()
				if len(toSend) == 0 {
					break
				}

				ssh.SetWriteDeadline(np.Now().Add(np.ConnWriteTimeout))
				w, err := ssh.Write(toSend)
				if err != nil {
					err = fmt.Errorf("connection write: %w", err)
					np.browser.Close(err)
					return err
				}
				wrecv.Empty(w)
				atomic.StoreUint32(&np.browserReadAck, uint32(wrecv.Emptied)&0xffffff)
			}
		}
	}
}

func (np *readWriter) Connect(ssh net.Conn, wc *websocket.Conn) waiter {
	go np.proxyToBrowser(ssh)
	go np.proxyFromBrowser(ssh)
	return np.browser.Set(wc, 0, 0)
}

func (np *readWriter) Attach(wc *websocket.Conn, wack, rack uint32) waiter {
	return np.browser.Set(wc, rack, wack)
}

// writeToBrowser blocks reading from the ssh connection, and writes to whichever
// is the current browser when the write function is invoked.
//
// Once data has been read from ssh, we cannot unread it.
//
// Further, we don't ever want to have more than one reader on the ssh connection,
// as which reader will succeed when will make it impossible to preserve the order
// of the data, short of even more complexity.
//
// Finally, there is no good mechanism to stop a read that's blocked for more data,
// unless we bypass the standard go library and use syscall select directly.
//
// The approach taken is that:
// 1) For each connection, we start a proxyToBrowser function as a goroutine.
// 2) If the browser goes away, the goroutine stops reading from the ssh session,
//    and waits for a browser to reconnect. This creates back pressure in the
//    TCP buffer, and should eventually slow down the sender, which is the desired
//    outcome.
// 3) The goroutine will only exit:
//    a) If there are errors on the ssh session (nothing we can do there).
//    b) If the browser goes away forever.
//    c) If the browser comes back, but for one or another we can't recover the
//       session (would happen if our buffers don't have enough data to sync back
//       with the browser).
func (np *readWriter) proxyToBrowser(ssh net.Conn) (err error) {
	defer func() {
		if err != nil {
			np.browser.log.Warnf("connection error %s", err)
			return
		}
	}()

	lastWriteAck := uint32(0)
	writeAckBuffer := [4]byte{}
	wsend := NewSendWindow(np.pool)

	defer ssh.Close()
	defer wsend.Drop()

	for {
		// TODO: we need to eventually kill the ssh session and client if we are idle for too long?
		// TODO: a client can send arbitrary amounts of data without ever acknowledging it, causing out of memory on the server.

		wack := atomic.LoadUint32(&np.browserWriteAck)
		if _, err := wsend.AcknowledgeUntil(wack); err != nil {
			err := fmt.Errorf("could not adjust send buffer - %w", err)
			np.browser.Close(err)
			return err
		}

		buffer := wsend.ToFill()
		forceSend := false
		ssh.SetReadDeadline(np.Now().Add(np.BrowserAckTimeout))
		read, err := ssh.Read(buffer)
		if err != nil {
			if terr, ok := err.(net.Error); !ok || !terr.Timeout() {
				err := fmt.Errorf("connection read returned: %w", err)
				np.browser.Close(err)
				return err
			}
		}
		wsend.Fill(read)

		currentAck := atomic.LoadUint32(&np.browserReadAck)
		if currentAck != lastWriteAck {
			forceSend = true
		}

		for {
			wc, rack, wack, err := np.browser.GetWack()
			if err != nil {
				return err
			}
			if wack != 0 {
				atomic.StoreUint32(&np.browserWriteAck, wack)
				if err := wsend.Reset(wack); err != nil {
					err := fmt.Errorf("client ack reset failed: %w", err)
					np.browser.Close(err)
					return err
				}
			}

			toWrite := wsend.ToEmpty()
			if len(toWrite) == 0 {
				if !forceSend {
					break
				}
			}
			forceSend = false

			writer, err := wc.NextWriter(websocket.BinaryMessage)
			if err != nil {
				np.browser.Error(wc, err)
				continue
			}

			wc.SetWriteDeadline(np.Now().Add(np.BrowserWriteTimeout))

			// Why? When a session resume, the client may end up resetting the browserReadAck value
			// to an older value, and resend some data. We don't want to send a stale browserReadAck
			// value, greater than it can handle.
			//
			// At the same time, browserReadAck is updated by a separate thread, we can't safely
			// overwrite it from here. So we stick to the ack the client asked for for as long as
			// we know the other thread has not processed it.
			if wack != 0 {
				lastWriteAck = rack
			} else {
				lastWriteAck = atomic.LoadUint32(&np.browserReadAck)
			}

			binary.BigEndian.PutUint32(writeAckBuffer[:], lastWriteAck&0xffffff)
			written, err := writer.Write(writeAckBuffer[:])
			if err != nil {
				np.browser.Error(wc, err)
				continue
			}
			if written != 4 {
				np.browser.Error(wc, fmt.Errorf("short write"))
				continue
			}

			written, err = writer.Write(toWrite)
			if err != nil {
				np.browser.Error(wc, err)
				continue
			}
			wsend.Empty(written)

			if err := writer.Close(); err != nil {
				np.browser.Error(wc, err)
				continue
			}
		}
	}
}

func (np *NasshProxy) ProxySsh(logid string, r *http.Request, w http.ResponseWriter, sid string, rack, wack uint32, hostport string) error {
	np.log.Infof("%s rack %08x wack %08x - connects %s", logid, rack, wack, hostport)

	rwi, found := np.connections.LoadOrStore(sid, newReadWriter(np.log, np.pool, np.timeouts))
	rw, converted := rwi.(*readWriter)
	if !converted {
		np.connections.Delete(sid)
		http.Error(w, "internal error in sid map", http.StatusInternalServerError)
		return fmt.Errorf("something went wrong, unexpected type in map")
	}

	if !found && rack != 0 && wack != 0 {
		np.connections.Delete(sid)
		http.Error(w, "request to resume connection, but sid is unknown", http.StatusGone)
		return fmt.Errorf("request to resume connection, but sid is unknown")
	}

	c, err := np.upgrader.Upgrade(w, r, nil)
	defer c.Close()

	if err != nil {
		http.Error(w, "failed to upgrade web socket", http.StatusBadRequest)
		return fmt.Errorf("failed to upgrade web socket %w", err)
	}

	var waiter waiter
	if !found {
		sshconn, err := net.DialTimeout("tcp", hostport, np.timeouts.ResolutionTimeout)
		if err != nil {
			return err
		}

		waiter = rw.Connect(sshconn, c)
	} else {
		waiter = rw.Attach(c, rack, wack)
	}

	err = waiter.Wait()
	var te *TerminatingError
	if errors.As(err, &te) {
		np.connections.Delete(sid)
		np.log.Warnf("%s terminating (forever) - rack %08x wack %08x", logid, rack, wack)
	} else {
		np.log.Warnf("%s terminating (retry-possible) - rack %08x wack %08x", logid, rack, wack)
	}
	return err
}

func (np *NasshProxy) Register(add MuxHandle) {
	add("/cookie", np.ServeCookie)
	add("/proxy", np.ServeProxy)
	add("/connect", np.ServeConnect)
}
