package nasshp

import (
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

	// sync.Pool of buffers to allocate and use for clients.
	pool *BufferPool

	// Key is a string, the SID. Value is a readWriter pointer.
	connections sync.Map
}

type options struct {
	bufferSize       int
	symmetricSetters []token.SymmetricSetter
	rng              *rand.Rand
}

type Flags struct {
	SymmetricKey []byte
	BufferSize   int
}

func DefaultFlags() *Flags {
	return &Flags{}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.ByteFileVar(&fl.SymmetricKey, prefix+"sid-encryption-key", "",
		"Path of the file containing the symmetric key to use to create/process sids. "+
			"If not supplied, a new key is generated")
	set.IntVar(&fl.BufferSize, prefix+"buffer-size", 8192, "Size of the buffers to use to send/receive data for connections")
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

func FromFlags(fl *Flags) Modifier {
	return func(np *NasshProxy, o *options) error {
		if len(fl.SymmetricKey) == 0 {
			key, err := token.GenerateSymmetricKey(o.rng, 0)
			if err != nil {
				return fmt.Errorf("the world is about to end, even random nubmer generators are failing - %w", err)
			}
			fl.SymmetricKey = key
		}
		WithSymmetricOptions(token.UseSymmetricKey(fl.SymmetricKey))(np, o)
		WithBufferSize(fl.BufferSize)(np, o)
		return nil
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

func New(rng *rand.Rand, authenticator oauth.Authenticate, mods ...Modifier) (*NasshProxy, error) {
	o := &options{rng: rng, bufferSize: 8192}
	np := &NasshProxy{
		authenticator: authenticator,
		log:           logger.Nil,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					return false
				}
				return strings.HasPrefix(origin, "chrome-extension://")
			},
		},
	}

	if err := Modifiers(mods).Apply(np, o); err != nil {
		return nil, err
	}

	if np.encoder == nil {
		be, err := token.NewSymmetricEncoder(rng, o.symmetricSetters...)
		if err != nil {
			return nil, fmt.Errorf("error setting up symmetric encryption: %w", err)
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
		Scheme: "chrome-extension",
		Path:   ext + "/" + path,
		// FIXME: actual url and port of gateway
		Fragment: "test@norad:9999",
	}

	if np.authenticator != nil {
		np.authenticator(w, r, target)
	} else {
		http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
	}
}

func (np *NasshProxy) ServeProxy(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	host := params.Get("host")
	port := params.Get("port")

	if np.authenticator != nil {
		np.authenticator(w, r, nil)
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

	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}

	sid, err := np.encoder.Encode([]byte(fmt.Sprintf("%s:%d", host, sp)))
	if err != nil {
		http.Error(w, "Sorry, the world is coming to an end, there was an error generating a session id. Good Luck.", http.StatusInternalServerError)
	}
	fmt.Fprintln(w, string(sid))
}

func (np *NasshProxy) ServeConnect(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	sid := params.Get("sid")
	ack := strings.TrimSpace(params.Get("ack"))
	pos := strings.TrimSpace(params.Get("pos"))

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

	hostportb, err := np.encoder.Decode([]byte(sid))
	if err != nil {
		http.Error(w, "invalid sid provided", http.StatusBadRequest)
		return
	}

	c, err := np.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to upgrade web socket: %s", err), http.StatusInternalServerError)
		return
	}

	err = np.ProxySsh(r, sid, rack, wack, string(hostportb), c)
	if err != nil {
		if err != io.EOF {
			np.log.Warnf("connection with %v dropped: %v", r.RemoteAddr, err)
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

	browser guardedBrowser
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
	ConnAckTimeout   time.Duration
}

type guardedBrowser struct {
	log logger.Logger

	notifier chan error
	wc       *websocket.Conn
	err      error

	pendingWack, pendingRack uint32

	lock sync.RWMutex
	cond *sync.Cond
}

func (gb *guardedBrowser) Init(log logger.Logger) {
	gb.log = log
	gb.cond = sync.NewCond(gb.lock.RLocker())
}

func (gb *guardedBrowser) GetWack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	wack := gb.pendingWack
	gb.pendingWack = 0
	return gb.wc, gb.pendingRack, wack, gb.err
}

func (gb *guardedBrowser) GetRack() (*websocket.Conn, uint32, uint32, error) {
	gb.cond.L.Lock() // This is a read only lock, see how cond is created.
	defer gb.cond.L.Unlock()
	for gb.wc == nil && gb.err == nil {
		gb.cond.Wait()
	}

	rack := gb.pendingRack
	gb.pendingRack = 0
	return gb.wc, rack, gb.pendingWack, gb.err
}

func (gb *guardedBrowser) Set(wc *websocket.Conn, rack, wack uint32) waiter {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	if gb.wc == wc {
		return gb.notifier
	}
	if gb.wc != nil {
		gb.notifier <- fmt.Errorf("replaced browser connection")
		gb.wc.Close()
	}
	gb.wc = wc
	if wc == nil {
		gb.notifier = nil
		gb.pendingRack = 0
		gb.pendingWack = 0
		return nil
	}

	gb.pendingRack = rack
	gb.pendingWack = wack
	gb.notifier = make(chan error, 1)
	gb.cond.Broadcast()
	return gb.notifier
}

type TerminatingError struct {
	error
}

func (gb *guardedBrowser) Close(err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()
	gb.err = err
	if gb.notifier != nil {
		gb.notifier <- &TerminatingError{error: err}
		gb.notifier = nil
	}
	if gb.wc != nil {
		gb.wc.Close()
		gb.wc = nil
	}
}

func (gb *guardedBrowser) Error(wc *websocket.Conn, err error) {
	gb.lock.Lock() // This is an exclusive write lock.
	defer gb.lock.Unlock()

	// The browser has already gone, nothing to do here.
	if gb.wc == nil || gb.wc != wc {
		return
	}

	gb.notifier <- err
	gb.wc.Close()
	gb.wc = nil
	gb.notifier = nil
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
			wrecv.Filled(read)
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

type waiter chan error

func (w waiter) Wait() error {
	return <-w
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
		if err := wsend.AcknowledgeUntil(wack); err != nil {
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
		wsend.Filled(read)

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

func (np *NasshProxy) ProxySsh(r *http.Request, sid string, rack, wack uint32, hostport string, c *websocket.Conn) error {
	np.log.Warnf("%s with %s starting - rack %08x wack %08x - connects %s", sid[0:6], r.RemoteAddr, rack, wack, hostport)
	timeouts := &Timeouts{
		Now: time.Now,

		// FIXME: read those from flags.
		BrowserWriteTimeout: time.Second * 60,
		BrowserAckTimeout:   time.Second * 1,
		ConnWriteTimeout:    time.Second * 60,
		ConnAckTimeout:      time.Second * 1,
		ResolutionTimeout:   time.Second * 30,
		// FIXME: add a cleanup timeout.
	}

	rwi, found := np.connections.LoadOrStore(sid, newReadWriter(np.log, np.pool, timeouts))
	rw, converted := rwi.(*readWriter)
	if !converted {
		np.connections.Delete(sid)
		return fmt.Errorf("something went wrong, unexpected type in map")
	}

	if !found && rack != 0 && wack != 0 {
		np.connections.Delete(sid)
		return fmt.Errorf("request to resume connection, but sid is unknown")
	}

	var waiter waiter
	if !found {
		sshconn, err := net.DialTimeout("tcp", hostport, timeouts.ResolutionTimeout)
		if err != nil {
			return err
		}

		// FIXME: have some mechanism to validate / resolve / check the host the client asked to connect to.
		// as it is today, the proxy can be used to connect _anywhere_, in or out your network.

		waiter = rw.Connect(sshconn, c)
	} else {
		waiter = rw.Attach(c, rack, wack)
	}

	err := waiter.Wait()
	var te *TerminatingError
	if errors.As(err, &te) {
		np.connections.Delete(sid)
		np.log.Warnf("%s terminating (forever) - rack %08x wack %08x", sid[0:6], rack, wack)
	} else {
		np.log.Warnf("%s terminating (retry-possible) - rack %08x wack %08x", sid[0:6], rack, wack)
	}
	return err
}

func (np *NasshProxy) Register(add MuxHandle) {
	add("/cookie", np.ServeCookie)
	add("/proxy", np.ServeProxy)
	add("/connect", np.ServeConnect)
}
