package nasshp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/proxy/utils"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	netLookupSRV = net.LookupSRV
)

type sessions struct {
	// Key is a string, the SID. Value is a readWriter pointer.
	sids sync.Map
	SessionCounters
}

func (s *sessions) Iterate(oneach func(rw *readWriter) bool) {
	s.sids.Range(func(key, value interface{}) bool {
		rw, converted := value.(*readWriter)
		if !converted {
			s.Invalid.Increment()
			return true
		}

		return oneach(rw)
	})
}

func (s *sessions) Get(sid string) *readWriter {
	rwi, _ := s.sids.Load(sid)
	if rwi == nil {
		return nil
	}

	// If it happens, something is wrong in sessions.
	rw, converted := rwi.(*readWriter)
	if !converted {
		s.Invalid.Increment()
		return nil
	}

	s.Resumed.Increment()
	return rw
}

func (s *sessions) Create(sid string, rw *readWriter) *readWriter {
	_, loaded := s.sids.LoadOrStore(sid, rw)
	// Should be incredibly rare. Client would need to be trying to
	// use the same sid in parallel. Handled in the caller.
	if loaded {
		return nil
	}

	s.Created.Increment()
	return rw
}

func (s *sessions) Delete(sid string) {
	s.sids.Delete(sid)
	s.Deleted.Increment()
}

func (s *sessions) Orphan(sid string) {
	s.Orphaned.Increment()
}

// getSRVPortResponse is marshalled to JSON and returned by the /get_srv_port
// handler.
type getSRVPortResponse struct {
	// Port for the queried hostname
	Port uint16 `json:"port"`
}

type NasshProxy struct {
	log           logger.Logger
	upgrader      websocket.Upgrader
	authenticator oauth.Authenticate
	encoder       token.BinaryEncoder
	filter        Filter

	// Timeouts to use, must be set.
	timeouts *Timeouts
	epolicy  *ExpirationPolicy

	// Where to redirect users after authentication to get their connections going.
	relayHost string

	// sync.Pool of buffers to allocate and use for clients.
	pool *BufferPool

	// Set of active sessions.
	sessions sessions

	// Error counters for http related events.
	errors   ProxyErrors
	counters ProxyCounters
	expires  ExpireCounters
}

func (np *NasshProxy) RelayHost() string {
	return np.relayHost
}

type options struct {
	bufferSize       int
	symmetricSetters []token.SymmetricSetter
	rng              *rand.Rand
}

type ExpirationPolicy struct {
	// How often to run session garbage collection.
	Every time.Duration

	// If number of orphaned sessions exceed this threshold, the oldest
	// orphaned sessions will be terminated until fewer than this limit are
	// left, no matter what. This is meant to be a last resort option.
	RuthlessThreshold int

	// If number of orphaned sessions exceed this threshold, the oldest
	// sessions that have been orphaned for longer than OrphanLimit are
	// expired until either the number of sessions goes below
	// OrphanThreshold, or there are no orphaned sessions that have been
	// around longer than OrphanLimit.
	//
	// Tl;Dr: this only expires the oldest sessions that have been orphaned
	// for longer than OrphanLimit.
	OrphanThreshold int
	OrphanLimit     time.Duration
}

func DefaultExpirationPolicy() *ExpirationPolicy {
	return &ExpirationPolicy{
		Every:             10 * time.Minute,
		RuthlessThreshold: 20000,
		OrphanThreshold:   1000,
		OrphanLimit:       3 * 24 * time.Hour * 24,
	}
}

func (ep *ExpirationPolicy) Register(set kflags.FlagSet, prefix string) {
	set.DurationVar(&ep.Every, prefix+"expire-run-every", ep.Every,
		"How often to check if there are sessions to be expired. Zero disables the check.")
	set.IntVar(&ep.RuthlessThreshold, prefix+"expire-ruthlessly-after", ep.RuthlessThreshold,
		"If more than this number of sessions are orphaned, the oldest sessions will be "+
			"expired until fewer than this limit sessions are left orphaned.")
	set.IntVar(&ep.OrphanThreshold, prefix+"expire-long-after", ep.OrphanThreshold,
		"If more than this number of sessions are orphaned, the oldest sessions that "+
			"have been orphaned for longer than expire-long-time will be expired.")
	set.DurationVar(&ep.OrphanLimit, prefix+"expire-long-time", ep.OrphanLimit,
		"Sessions orphaned for less than this time will not be expired unless the "+
			"ruthless limit is exceeded")
}

func (ep *ExpirationPolicy) Expire(ctx context.Context, clock utils.Clock, sessions *sessions, counters *ExpireCounters) {
	var start time.Time
	for {
		counters.ExpireRuns.Increment()
		if !start.IsZero() {
			end := clock.Now()
			counters.ExpireDuration.Add(uint64(end.Sub(start)))
		}

		select {
		case <-ctx.Done():
			return

		case <-clock.After(ep.Every):
		}

		start = clock.Now()

		total := sessions.Created.Get() - sessions.Deleted.Get()
		if total < uint64(ep.OrphanThreshold) {
			continue
		}

		counters.ExpireAboveOrphanThresholdRuns.Increment()
		counters.ExpireAboveOrphanThresholdTotal.Add(total)

		// Save the time BEFORE we start scanning sessions (so we can
		// exclude all sessions (un)paused after the scan), and find
		// all sessions paused.
		now := clock.Now().UnixNano()
		orphaned := []*readWriter{}
		sessions.Iterate(func(rw *readWriter) bool {
			pausedat := rw.browser.paused.Nano()
			if pausedat <= 0 || pausedat > now {
				return true
			}

			orphaned = append(orphaned, rw)
			return true
		})

		counters.ExpireAboveOrphanThresholdFound.Add(uint64(len(orphaned)))
		if len(orphaned) < ep.OrphanThreshold {
			continue
		}

		sort.Slice(orphaned, func(i, j int) bool {
			return orphaned[i].browser.paused.Nano() < orphaned[j].browser.paused.Nano()
		})

		maxorphan := int64(ep.OrphanLimit * time.Nanosecond)
		for cursor, left := 0, len(orphaned); cursor < len(orphaned) && left > ep.OrphanThreshold; cursor, left = cursor+1, left-1 {
			rw := orphaned[cursor]

			// This can happen if the session was unpaused in
			// between scanning the map and processing the list of
			// sessions. No matter what, there's a short race here.
			pausedon := rw.browser.paused.Nano()
			pausedfor := now - pausedon
			if pausedon <= 0 || pausedfor <= 0 {
				counters.ExpireRaced.Increment()
				continue
			}

			if left <= ep.RuthlessThreshold {
				if pausedfor <= maxorphan {
					break
				}
				counters.ExpireOrphanClosed.Increment()
			} else {
				counters.ExpireRuthlessClosed.Increment()
			}

			counters.ExpireLifetimeTotal.Add(uint64(pausedfor) / uint64(time.Second))
			counters.ExpireYoungest.SetIfGreatest(uint64(pausedon))

			rw.browser.Close(fmt.Errorf(
				"session expired after %s of inactivity", time.Duration(pausedfor)))
		}
	}
}

type Flags struct {
	*Timeouts
	*ExpirationPolicy

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
	}
}

func DefaultFlags() *Flags {
	return &Flags{
		Timeouts:         DefaultTimeouts(),
		ExpirationPolicy: DefaultExpirationPolicy(),
	}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.ByteFileVar(&fl.SymmetricKey, prefix+"sid-encryption-key", "",
		"Path of the file containing the symmetric key to use to create/process sids. "+
			"If not supplied, a new key is generated")
	set.IntVar(&fl.BufferSize, prefix+"buffer-size", 8192, "Size of the buffers to use to send/receive data for connections")
	set.StringVar(&fl.RelayHost, prefix+"host-port", "", "The hostname and port number the nassh client has to be redirected to to establish an ssh connection. "+
		"Typically, this is the DNS name and port 80 or 443 of the host running this proxy.")

	set.DurationVar(&fl.BrowserWriteTimeout, prefix+"browser-write-timeout", fl.BrowserWriteTimeout,
		"How long to wait for a write to the browser to complete before giving up.")
	set.DurationVar(&fl.BrowserAckTimeout, prefix+"browser-ack-timeout", fl.BrowserAckTimeout,
		"How long to wait before sending an ack back.")
	set.DurationVar(&fl.ConnWriteTimeout, prefix+"conn-write-timeout", fl.ConnWriteTimeout,
		"How long to wait for a write to the proxied connection (eg, ssh) to complete before giving up.")
	set.DurationVar(&fl.ResolutionTimeout, prefix+"resolution-timeout", fl.ResolutionTimeout,
		"How long to wait to resolve the name of the destination of the proxied connection")

	fl.ExpirationPolicy.Register(set, prefix)
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

type Filter func(proto string, hostport string, creds *oauth.CredentialsCookie) utils.Verdict

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
			WithExpirationPolicy(fl.ExpirationPolicy),
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

func WithExpirationPolicy(ep *ExpirationPolicy) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.epolicy = ep
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
		epolicy:       DefaultExpirationPolicy(),
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

func (np *NasshProxy) requestError(counter *utils.Counter, w http.ResponseWriter, format string, args ...interface{}) {
	np.requestErrorStatus(counter, w, http.StatusBadRequest, format, args...)
}

func (np *NasshProxy) requestErrorStatus(counter *utils.Counter, w http.ResponseWriter, status int, format string, args ...interface{}) {
	counter.Increment()
	http.Error(w, fmt.Sprintf(format, args...), status)
}

func (np *NasshProxy) ServeCookie(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	ext := params.Get("ext")
	path := params.Get("path")
	if ext == "" || path == "" {
		np.requestError(&np.errors.CookieInvalidParameters, w, "invalid request for: %s", r.URL)
		return
	}

	target := &url.URL{
		Scheme:   "chrome-extension",
		Path:     ext + "/" + path,
		Fragment: "nasshp-enkit@" + np.relayHost,
	}

	if !np.authenticate(w, r, &np.errors.CookieInvalidAuth) {
		return
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
	if !np.authenticate(w, r, &np.errors.ProxyInvalidAuth) {
		return
	}

	sp, err := strconv.ParseUint(port, 10, 16)
	if err != nil || port == "" {
		np.requestError(&np.errors.ProxyInvalidPort, w, "invalid port requested: %s", port)
		return
	}
	if host == "" {
		np.requestError(&np.errors.ProxyInvalidHost, w, "invalid empty host: %s", host)
		return
	}
	hostport := net.JoinHostPort(host, strconv.Itoa(int(sp)))

	_, allowed := np.allow(&np.errors.ProxyAllow, r, w, "", hostport)
	if !allowed {
		return
	}

	sid, err := np.encoder.Encode([]byte(hostport))
	if err != nil {
		np.requestErrorStatus(&np.errors.ProxyCouldNotEncrypt, w, http.StatusInternalServerError,
			"Sorry, the world is coming to an end, there was an error generating a session id. Good Luck.")
	}
	fmt.Fprintln(w, string(sid))
}

// GetSRVPort looks up an SRV DNS record for the specified host and returns the
// first record's port number, or an error if no SRV records exist for that
// host.
func (np *NasshProxy) GetSRVPort(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	host := params.Get("host")

	if !np.authenticate(w, r, &np.errors.SrvLookupInvalidAuth) {
		return
	}

	_, srvs, err := netLookupSRV("", "", host)
	if err != nil {
		np.requestError(&np.errors.SrvLookupFailed, w, "failed to lookup SRV records for %q: %v", host, err)
		return
	}
	if len(srvs) < 1 {
		np.requestError(&np.errors.SrvLookupFailed, w, "no SRV records for %q", host, err)
		return
	}
	res := getSRVPortResponse{
		Port: srvs[0].Port,
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		np.log.Errorf("Failed to marshal %T %+v to JSON: %v", res, res, err)
		return
	}
	return
}

func (np *NasshProxy) authenticate(w http.ResponseWriter, r *http.Request, errCounter *utils.Counter) bool {
	if np.authenticator == nil {
		return true
	}
	creds, err := np.authenticator(w, r, nil)
	if err != nil {
		np.requestError(errCounter, w, "invalid request for: %s - %s", r.URL, err)
		return false
	}
	if creds == nil {
		// There are no credentials, so the user has been redirected to the
		// authentication service.
		return false
	}
	return true
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

func (np *NasshProxy) allow(counters *AllowErrors, r *http.Request, w http.ResponseWriter, sid, hostport string) (string, bool) {
	logid := LogId(sid, r, hostport, nil)

	var creds *oauth.CredentialsCookie
	if np.authenticator != nil {
		// TODO: merge credentials checking with filtering mechanism.
		creds, err := np.authenticator(w, r, nil)
		if err != nil {
			np.log.Warnf("%s - authentication error: %s", logid, err)
			np.requestError(&counters.InvalidCookie, w, "invalid request for: %s - %s", r.URL, err)
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
			np.requestErrorStatus(
				&counters.InvalidHostFormat, w, http.StatusUnauthorized,
				"Go somewhere else, you are not allowed to connect here.")
			return logid, false
		}
		res, err := net.LookupHost(host)
		if err != nil {
			np.log.Infof("%s - err %v looking up host %s", logid, err, host)
			np.requestErrorStatus(
				&counters.InvalidHostName, w, http.StatusUnauthorized,
				"Go somewhere else, you are not allowed to connect here.")
			return logid, false
		}
		verdict := utils.VerdictUnknown
		for _, u := range res {
			// TODO(adam): make verdict merging configurable from ACL list
			// TODO(adam): return here after making authz engine
			verdict = verdict.MergeOnlyAcceptAllow(np.filter("tcp", net.JoinHostPort(u, port), creds))
		}
		if verdict == utils.VerdictAllow {
			return logid, true
		}

		np.log.Infof("%s was rejected by filter", logid)
		np.requestErrorStatus(
			&counters.Unauthorized, w, http.StatusUnauthorized,
			"Go somewhere else, you are not allowed to connect here.")
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
		np.requestError(&np.errors.ConnectInvalidSID, w, "invalid sid provided")
		return
	}
	hostport := string(hostportb)

	logid := LogId(sid, r, hostport, nil)

	logid, allow := np.allow(&np.errors.ConnectAllow, r, w, sid, hostport)
	if !allow {
		return
	}
	np.log.Infof("%s - connect allowed", logid)

	rack := uint32(0)
	if ack != "" {
		sp, err := strconv.ParseUint(ack, 10, 32)
		if err != nil {
			np.requestError(&np.errors.ConnectInvalidAck, w, "invalid ack requested: %s - %s", ack, err)
			return
		}
		rack = uint32(sp)
	}

	wack := uint32(0)
	if pos != "" {
		sp, err := strconv.ParseUint(pos, 10, 32)
		if err != nil {
			np.requestError(&np.errors.ConnectInvalidPos, w, "invalid pos requested: %s - %s", pos, err)
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
	*ReadWriterCounters

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

func newReadWriter(log logger.Logger, pool *BufferPool, timeouts *Timeouts, counters *ReadWriterCounters) *readWriter {
	rw := &readWriter{ReadWriterCounters: counters, Timeouts: timeouts, pool: pool}
	rw.browser.Init(log, &counters.BrowserWindowCounters)
	return rw
}

type Timeouts struct {
	Now utils.TimeSource

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
	np.BrowserReaderStarted.Increment()
	defer func() {
		np.BrowserReaderStopped.Increment()
		if err != nil {
			np.BrowserReaderError.Increment()
			np.browser.log.Warnf("browser error %s", err)
			return
		}
	}()

	ackbuffer := [4]byte{}

	wrecv := NewReceiveWindow(np.pool)

	defer ssh.Close()
	defer wrecv.Drop()

retry:
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

		for ackread := 0; ackread < len(ackbuffer); {
			read, err := browser.Read(ackbuffer[ackread:])
			if err != nil {
				if err != io.EOF {
					np.browser.Error(wc, err)
				}
				continue retry
			}
			np.BrowserBytesRead.Add(uint64(read))
			ackread += read
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
			np.BrowserBytesRead.Add(uint64(read))
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
				np.BackendBytesWrite.Add(uint64(w))
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
	np.BrowserWriterStarted.Increment()
	defer func() {
		np.BrowserWriterStopped.Increment()
		if err != nil {
			np.BrowserWriterError.Increment()
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
		np.BackendBytesRead.Add(uint64(read))

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
			np.BrowserBytesWrite.Add(uint64(written))
			if written != 4 {
				np.browser.Error(wc, fmt.Errorf("short write"))
				continue
			}

			written, err = writer.Write(toWrite)
			if err != nil {
				np.browser.Error(wc, err)
				continue
			}

			np.BrowserBytesWrite.Add(uint64(written))
			wsend.Empty(written)

			if err := writer.Close(); err != nil {
				np.browser.Error(wc, err)
				continue
			}
		}
	}
}

// Run starts nasshp background workers.
//
// It should typically be started in a goroutine of its own.
//
// It completes eiter when there are no workers to be run, or
// when the supplied ctx is canceled.
func (np *NasshProxy) Run(ctx context.Context) {
	if np.epolicy == nil {
		return
	}

	if np.epolicy.Every != 0 {
		np.epolicy.Expire(ctx, &utils.SystemClock{}, &np.sessions, &np.expires)
	}
}

func (np *NasshProxy) ProxySsh(logid string, r *http.Request, w http.ResponseWriter, sid string, rack, wack uint32, hostport string) error {
	np.counters.SshProxyStarted.Increment()
	defer np.counters.SshProxyStopped.Increment()
	np.log.Infof("%s rack %08x wack %08x - connects %s", logid, rack, wack, hostport)

	rw := np.sessions.Get(sid)
	if rw == nil && (rack != 0 || wack != 0) {
		np.requestErrorStatus(&np.errors.SshResumeNoSID, w, http.StatusGone, "request to resume connection, but sid is unknown")
		return fmt.Errorf("request to resume connection, but sid is unknown")
	}

	// Watch out: after upgrade, can't change the http status anymore!
	c, err := np.upgrader.Upgrade(w, r, nil)
	defer c.Close()

	if err != nil {
		np.errors.SshFailedUpgrade.Increment()
		return fmt.Errorf("failed to upgrade web socket %w", err)
	}

	var waiter waiter
	if rw == nil {
		sshconn, err := net.DialTimeout("tcp", hostport, np.timeouts.ResolutionTimeout)
		if err != nil {
			np.errors.SshDialFailed.Increment()
			return err
		}

		rw = np.sessions.Create(sid, newReadWriter(np.log, np.pool, np.timeouts, &np.counters.ReadWriterCounters))
		if rw == nil {
			np.errors.SshCreateExists.Increment()
			sshconn.Close()
			return fmt.Errorf("request to start new session, but sid exists already")
		}

		waiter = rw.Connect(sshconn, c)
	} else {
		waiter = rw.Attach(c, rack, wack)
	}

	err = waiter.Wait()
	var te *TerminatingError
	if errors.As(err, &te) {
		np.sessions.Delete(sid)
		np.log.Warnf("%s terminating (forever) - rack %08x wack %08x", logid, rack, wack)
	} else {
		np.sessions.Orphan(sid)
		np.log.Warnf("%s terminating (retry-possible) - rack %08x wack %08x", logid, rack, wack)
	}
	return err
}

// MuxHandle is a function capable of instructing an http Mux to invoke an handler for a path.
//
// pattern is a string representing a path without host (example: "/", or "/test").
// No wildcards or field extraction is used by nasshp, only constants need to be supported
// by MuxHandle.
//
// handler is the http.Handler to invoke for the specified path.
type MuxHandle func(pattern string, handler http.Handler)

// Register is a convenience function to configure all the handlers in your favourite Mux.
//
// It configures the http paths and corresponding handlers that are necessary for
// a nassh implementation to support.
//
// Registering the paths can also be done manually. Rather than document the required paths
// in comments here,look at the source code of the function.
func (np *NasshProxy) Register(add MuxHandle) {
	add("/cookie", http.HandlerFunc(np.ServeCookie))
	add("/proxy", http.HandlerFunc(np.ServeProxy))
	add("/connect", http.HandlerFunc(np.ServeConnect))
	add("/get_srv_port", http.HandlerFunc(np.GetSRVPort))
}

func (np *NasshProxy) ExportMetrics(register prometheus.Registerer) error {
	return register.Register((*nasshCollector)(np))
}
