// Package ktransport provides modifiers to create and work with http.Transport
// and http2.Transport objects.
//
// You can pass those modifiers to other functions in the khttp library, or use
// them to create a new http or http2 transport object via NewTransport or
// NewTransport2.
//
// For example, to create an http transport object with a well defined TLS handshake
// timeout and a specific set of TLS settings, you can use:
//
//	transport, err := ktransport.NewHTTP(
//	    ktransport.WithTLSHandshakeTimeout(10 * time.Second),
//	    ktransport.WithTLSOptions(
//	        ktls.WithRootCAFile("/etc/corp/enfabrica.crt"),
//	    )
//	)
//
// To have a transport of choice based on command line flags, you can use the
// RTFlags object (RT for RoundTripper), and corresponding New method.
package ktransport

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/ktls"
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"time"
)

// Modifier applies options to a plain http.Transport.
type Modifier func(transport *http.Transport) error

type Modifiers []Modifier

// Apply applies the set of modifiers to the specified config.
func (mods Modifiers) Apply(transport *http.Transport) error {
	for _, m := range mods {
		if err := m(transport); err != nil {
			return err
		}
	}
	return nil
}

// Modifier2 applies options to an http2.Transport.
type Modifier2 func(transport *http2.Transport) error

type Modifiers2 []Modifier2

// Apply applies the set of modifiers to the specified config.
func (mods Modifiers2) Apply(transport *http2.Transport) error {
	for _, m := range mods {
		if err := m(transport); err != nil {
			return err
		}
	}
	return nil
}

// NewHTTP returns a default transport with the options supplied applied.
//
// At time of writing (2022), the default http transport for go supports
// HTTP/1.1, and if configured for HTTPS, it allows for HTTP/2 upgrade if
// both the client and server support it.
//
// The returned *http.Transport implements the http.RoundTripper interface, and
// can be used directly in kclient.WithTransport(ktransport.NewHTTP(...)).
func NewHTTP(mods ...Modifier) (*http.Transport, error) {
	t := &http.Transport{}
	if err := Modifiers(mods).Apply(t); err != nil {
		return nil, err
	}
	return t, nil
}

// NewHTTP1 returns a transport that only supports HTTP < 2.
//
// Unless you need to explicitly disallow the use of HTTP/2, you
// should just use NewHTTP() instead.
func NewHTTP1(mods ...Modifier) (*http.Transport, error) {
	return NewHTTP(append([]Modifier{WithHTTP2Disabled()}, mods...)...)
}

// NewHTTP2 returns a transpoort that only supports HTTP/2.
//
// This is also known as HTTP/2 prior knowledge, as both the
// client and server MUST be capable of using HTTP/2. If not,
// the connection will fail.
//
// The returned *http2.Transport implements the http.RoundTripper interface,
// and can be used directly in kclient.WithTransport(ktransport.NewHTTP2(...)).
func NewHTTP2(mods ...Modifier2) (*http2.Transport, error) {
	t := &http2.Transport{}

	if err := Modifiers2(mods).Apply(t); err != nil {
		return nil, err
	}
	return t, nil
}

// NewH2C returns a transport that supports H2C only.
//
// The server must also support H2C or the connection will fail.
//
// The returned *http2.Transport implements the http.RoundTripper interface,
// and can be used directly in kclient.WithTransport(ktransport.NewH2C(...)).
func NewH2C(mods ...Modifier2) (*http2.Transport, error) {
	return NewHTTP2(append([]Modifier2{WithH2COnly2()}, mods...)...)
}

// Flags defines the command line tunables for an http.Transport.
type Flags struct {
	ExpectContinueTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
}

func DefaultFlags() *Flags {
	flags := &Flags{}

	transport, ok := http.DefaultTransport.(*http.Transport)

	// Goal is to use the timeouts set in the default transport by default.
	// If that transport does not exist, or is not net.http, then ... let's use
	// the language default, same as if we used an empty object.
	if ok {
		flags.ExpectContinueTimeout = transport.ExpectContinueTimeout
		flags.TLSHandshakeTimeout = transport.TLSHandshakeTimeout
		flags.IdleConnTimeout = transport.IdleConnTimeout
		flags.MaxIdleConns = transport.MaxIdleConns
	}

	return flags
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.DurationVar(&fl.ExpectContinueTimeout, prefix+"http-expect-continue-timeout",
		fl.ExpectContinueTimeout, "How long to wait for a continue in a persistent http connection")
	set.DurationVar(&fl.TLSHandshakeTimeout, prefix+"http-tls-handshake-timeout",
		fl.TLSHandshakeTimeout, "How long to wait for the TLS Handshke to complete")
	set.DurationVar(&fl.IdleConnTimeout, prefix+"http-idle-conn-timeout",
		fl.IdleConnTimeout, "How long to keep a connection open before closing it")
	set.IntVar(&fl.MaxIdleConns, prefix+"http-max-idle-conns",
		fl.MaxIdleConns, "How many idle connections to keep at most")
	return fl
}

// Matches can be used to check if the flags configured actually change the transport.
//
// Returns true if the flags match the configured object, false otherwise.
func (fl *Flags) Matches(transport *http.Transport) bool {
	if transport.ExpectContinueTimeout != fl.ExpectContinueTimeout || transport.TLSHandshakeTimeout != fl.TLSHandshakeTimeout || transport.IdleConnTimeout != fl.IdleConnTimeout {
		return false
	}
	if transport.MaxIdleConns != fl.MaxIdleConns {
		return false
	}

	return true
}

// FromFlags initializes a transport from the supplied flags object.
func FromFlags(flags *Flags) Modifier {
	return func(transport *http.Transport) error {
		if flags == nil {
			return nil
		}

		transport.ExpectContinueTimeout = flags.ExpectContinueTimeout
		transport.TLSHandshakeTimeout = flags.TLSHandshakeTimeout
		transport.IdleConnTimeout = flags.IdleConnTimeout
		transport.MaxIdleConns = flags.MaxIdleConns

		return nil
	}
}

// Flags2 defines the command line tunables for an http2.Transport.
type Flags2 struct {
	ReadIdleTimeout  time.Duration
	PingTimeout      time.Duration
	WriteByteTimeout time.Duration

	CompressionEnabled bool
}

func DefaultFlags2() *Flags2 {
	return &Flags2{CompressionEnabled: true}
}

func (fl *Flags2) Register(set kflags.FlagSet, prefix string) *Flags2 {
	set.DurationVar(&fl.ReadIdleTimeout, prefix+"http2-read-idle-timeout", fl.ReadIdleTimeout,
		"If set, a health check is performed once the timeout expires without frames on the connection")
	set.DurationVar(&fl.PingTimeout, prefix+"http2-ping-timeout", fl.PingTimeout,
		"If no response is received to a health check after this timeout, the connection is closed")
	set.DurationVar(&fl.WriteByteTimeout, prefix+"http2-write-byte-timeout", fl.WriteByteTimeout,
		"If there is pending data to write, and no more data can be written within the timeout, the connection is closed")
	set.BoolVar(&fl.CompressionEnabled, prefix+"http2-compression-enabled", fl.CompressionEnabled,
		"If set to true, http/2 compression is enabled")
	return fl
}

// Matches can be used to check if the flags configured actually change the transport.
//
// Returns true if the flags match the configured object, false otherwise.
func (fl *Flags2) Matches(transport *http2.Transport) bool {
	if transport.ReadIdleTimeout != fl.ReadIdleTimeout || transport.PingTimeout != fl.PingTimeout || transport.WriteByteTimeout != fl.WriteByteTimeout {
		return false
	}
	if transport.DisableCompression != !fl.CompressionEnabled {
		return false
	}

	return true
}

// FromFlags initializes a transport from the supplied flags object.
func FromFlags2(flags *Flags2) Modifier2 {
	return func(transport *http2.Transport) error {
		if flags == nil {
			return nil
		}

		transport.ReadIdleTimeout = flags.ReadIdleTimeout
		transport.PingTimeout = flags.PingTimeout
		transport.WriteByteTimeout = flags.WriteByteTimeout
		transport.DisableCompression = !flags.CompressionEnabled

		return nil
	}
}

// RTFlags allows to create a generic transport (RoundTripper) based on command line flags.
//
// To use it:
//  1. Create a RTFlags object using DefaultRTFlags().
//  2. Tweak any default that needs to be changed in your code.
//  3. Call .Register() on the RTFlags object.
//  4. After flag parsing, use New(ff, ...)
type RTFlags struct {
	HTTP  *Flags
	HTTP2 *Flags2
	TLS   *ktls.Flags

	Dialect string
}

func DefaultRTFlags() *RTFlags {
	return &RTFlags{
		HTTP:  DefaultFlags(),
		HTTP2: DefaultFlags2(),
		TLS:   ktls.DefaultFlags(),

		Dialect: "default",
	}
}

// Register flags needed by the RTFlags object.
func (fl *RTFlags) Register(set kflags.FlagSet, prefix string) *RTFlags {
	fl.HTTP.Register(set, prefix)
	fl.HTTP2.Register(set, prefix)
	fl.TLS.Register(set, prefix)

	set.StringVar(&fl.Dialect, prefix+"http-dialect", fl.Dialect,
		"HTTP dialect to use - one of: default (or empty), http1, http2, h2c. default: will start with http1.*, upgrade to http2 if possible. "+
			"http1: starts with http1.*, does not upgrade to http2. http2: requires https url, starts with http2, fails if not supported. "+
			"h2c: requires http url, starts with http2 on a cleartext connection.")

	return fl
}

type RTModifier func(*Modifiers, *Modifiers2, *ktls.Modifiers)

type RTModifiers []RTModifier

// Apply applies the modifiers in RTModifiers to the supplied RoundTripper.
//
// If the RoundTripper is an http2.Transport, http.Transport parameters are ignored.
//
// If the RoundTripper is an http.Transport with http2 enabled, http2.Transport
// parameters will result in WithHTTP2Options being invoked, explicitly configuring
// the http2.Transport parameters for http.Transport in case an upgrade is negotiated.
// Note that WithHTTP2Options can only be invoked once on a transport due to golang,
// API constraints, so make sure that all the necessary mods are in place
// before invoking Apply.
//
// TLS Options are applied to the transport that will be used to establish the
// HTTP connection only (http.Transport object, or http2.Transport only in case
// http2 prior knowledge is configured).
func (mods RTModifiers) Apply(rt http.RoundTripper) error {
	h1 := Modifiers{}
	h2 := Modifiers2{}
	tm := ktls.Modifiers{}
	for _, m := range mods {
		m(&h1, &h2, &tm)
	}

	// The TLS options only affect the outer transport used.
	// Eg, when an http2 upgrade is performed, TLS is already established.
	// No need to change the TLS config in WithHTTP2Options().
	switch t := rt.(type) {
	case *http.Transport:
		if len(tm) > 0 {
			h1 = append(h1, WithTLSOptions(tm...))
		}
		if len(h2) > 0 {
			h1 = append(h1, WithHTTP2Options(h2...))
		}
		return h1.Apply(t)

	case *http2.Transport:
		if len(tm) > 0 {
			h2 = append(h2, WithTLSOptions2(tm...))
		}
		return h2.Apply(t)

	default:
	}

	return fmt.Errorf("unknonw transport type: %T", rt)
}

// WithRTOptions adds HTTP options.
//
// The options are used only if an HTTP or HTTP1 transport is created.
// The parameters are otherwise ignored.
func WithRTOptions(mods ...Modifier) RTModifier {
	return func(m *Modifiers, _ *Modifiers2, _ *ktls.Modifiers) {
		*m = append(*m, mods...)
	}
}

// WithRTOptions adds HTTP2 options.
//
// The options are used only if an HTTP2 transport is created.
// The parameters are otherwise ignored.
func WithRTOptions2(mods ...Modifier2) RTModifier {
	return func(_ *Modifiers, m *Modifiers2, _ *ktls.Modifiers) {
		*m = append(*m, mods...)
	}
}

// WithRTTLSOptions adds TLS options.
//
// The options are used only if TLS is used.
func WithRTTLSOptions(mods ...ktls.Modifier) RTModifier {
	return func(_ *Modifiers, _ *Modifiers2, t *ktls.Modifiers) {
		*t = append(*t, mods...)
	}
}

// WithRTFlags applies the options specified by the RTFlags object.
func WithRTFlags(flags *RTFlags) RTModifier {
	return func(m1 *Modifiers, m2 *Modifiers2, tm *ktls.Modifiers) {
		*m1 = append(*m1, FromFlags(flags.HTTP))
		*tm = append(*tm, ktls.FromFlags(flags.TLS))

		// Adding an http2 modifier forces the creation of a dedicated
		// http2.Transport. Let's avoid it unless flags require it.
		if !flags.HTTP2.Matches(&http2.Transport{}) {
			*m2 = append(*m2, FromFlags2(flags.HTTP2))
		}
	}
}

// NewByDialect returns a new transport based on a string identifying the type.
//
// If NewByDialect is invoked with an empty string or "default", it returns a NewHTTP().
// If invoked with "http1", it returns a NewHTTP1().
// If invoked with "http2", it returns a NewHTTP2().
// If invoked with "h2c", it returns a NewH2C().
//
// In any other case, it returns nil.
func NewByDialect(dialect string) (http.RoundTripper, error) {
	switch dialect {
	case "default":
		fallthrough
	case "":
		return NewHTTP()

	case "http1":
		return NewHTTP1()

	case "http2":
		return NewHTTP2()

	case "h2c":
		return NewH2C()
	}

	return nil, fmt.Errorf("invalid dialect %s - valid: default, http1, http2, h2c", dialect)
}

// CanUseDefault returns true if the golang http.DefaultTransport is deemed safe to use.
//
// This function returns true if the supplied flags and modifiers are not expected
// to introduce changes in the DefaultTransport configuration, thus allowing the DefaultTransport
// to be used as configured by the user.
//
// This can potentially allow better connection pooling and re-use depending on client code.
func CanUseDefault(fl *RTFlags, mods ...RTModifier) bool {
	tm := fl.TLS.Modifiers()
	if (fl.Dialect == "default" || fl.Dialect == "") && len(mods) == 0 && len(tm) == 0 && fl.HTTP2.Matches(&http2.Transport{}) {
		transport, ok := http.DefaultTransport.(*http.Transport)

		// TLSNextProto != nil may mean that http2 has been explicitly enabled/disabled,
		// thus invalidating the ...Matches(&http2.Transport{}) in the if above.
		if ok && fl.HTTP.Matches(transport) && transport.TLSNextProto == nil {
			return true
		}
	}
	return false
}

// DefaultOrNew returns a transport based on the supplied flags and modifiers.
//
// If the flags and modifiers would create a transport equivalent to the http.DefaultTransport,
// the http.DefaultTransport is returned instead.
//
// If not, a new transport is created.
//
// The modifiers supplied are applied as defaults, the flags supplied will
// override them. You can separately use RTModifiers.Apply() on the returned
// object to apply further parameters, or manually invoke CanUseDefault() and
// New() to implement arbitrary semantics.
func DefaultOrNew(fl *RTFlags, mods ...RTModifier) (http.RoundTripper, error) {
	if CanUseDefault(fl, mods...) {
		return http.DefaultTransport, nil
	}

	return New(fl.Dialect, append(RTModifiers{WithRTFlags(fl)}, mods...)...)
}

// New creates a new transport speaking the specified dialect configured as per modifiers.
func New(dialect string, mods ...RTModifier) (http.RoundTripper, error) {
	rt, err := NewByDialect(dialect)
	if err != nil {
		return nil, kflags.NewUsageErrorf("--http-dialect invalid: %w", err)
	}

	if err := RTModifiers(mods).Apply(rt); err != nil {
		return nil, err
	}

	return rt, nil
}

// WithExpectContinueTimeout configures an http.Transport ExpectContinueTimeout.
func WithExpectContinueTimeout(timeout time.Duration) Modifier {
	return func(transport *http.Transport) error {
		transport.ExpectContinueTimeout = timeout
		return nil
	}
}

// WithTLSHandshakeTimeout configures an http.Transport TLSHandshakeTimeout.
func WithTLSHandshakeTimeout(timeout time.Duration) Modifier {
	return func(transport *http.Transport) error {
		transport.TLSHandshakeTimeout = timeout
		return nil
	}
}

// WithIdleConnTimeout configures an http.Transport IdleConnTimeout.
func WithIdleConnTimeout(timeout time.Duration) Modifier {
	return func(transport *http.Transport) error {
		transport.IdleConnTimeout = timeout
		return nil
	}
}

// WithMaxIdleConns configures an http.Transport MaxIdleConns.
func WithMaxIdleConns(value int) Modifier {
	return func(transport *http.Transport) error {
		transport.MaxIdleConns = value
		return nil
	}
}

// WithForceAttemptHTTP2 configures an http.Transport ForceAttemptHTTP2.
//
// According to go docs, you should ForceAttemptHTTP2 whenever the Dial,
// DialTLS, DialContext, or TLSClientConfig fields are provided, and HTTP2
// should still be attempted.
func WithForceAttemptHTTP2(value bool) Modifier {
	return func(transport *http.Transport) error {
		transport.ForceAttemptHTTP2 = value
		return nil
	}
}

// WithTLSConfig adds a tls client configuration.
//
// IMPORTANT: setting a TLS config in an http.Transport implicitly disables
// upgrading the connection to http2. If that's not intended, you should
// use WithForceAttemptHTTP2(true).
func WithTLSConfig(config *tls.Config) Modifier {
	return func(transport *http.Transport) error {
		transport.TLSClientConfig = config
		return nil
	}
}

// WithTLSOptions applies the tls modifiers to the client configuration.
//
// IMPORTANT: setting TLS options in an http.Transport implicitly disables
// upgrading the connection to http2. If that's not intended, you should
// use WithForceAttemptHTTP2(true).
func WithTLSOptions(mods ...ktls.Modifier) Modifier {
	return func(transport *http.Transport) error {
		config := transport.TLSClientConfig
		if config == nil {
			config = &tls.Config{}
		} else {
			config = config.Clone()
		}

		if err := ktls.Modifiers(mods).Apply(config); err != nil {
			return err
		}

		transport.TLSClientConfig = config
		return nil
	}
}

// WithH2COnly configures the transport to use H2C (http/2 over cleartext).
//
// Due to constrains in the current implementation, enabling H2C disables the
// ability to establish https connections over http2.
//
// It is not tested/not guaranteed to work outside of an http/2 prior knowledge
// transport (eg, a transport created with NewHTTP2().
func WithH2COnly2() Modifier2 {
	return func(transport *http2.Transport) error {
		transport.AllowHTTP = true
		transport.DialTLS = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}
		transport.DialTLSContext = func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}

		return nil
	}
}

// WithTLSConfig2 adds a tls client configuration.
func WithTLSConfig2(config *tls.Config) Modifier2 {
	return func(transport *http2.Transport) error {
		transport.TLSClientConfig = config
		return nil
	}
}

// WithTLSOptions2 applies the tls modifiers to the client configuration.
func WithTLSOptions2(mods ...ktls.Modifier) Modifier2 {
	return func(transport *http2.Transport) error {
		config := transport.TLSClientConfig
		if config == nil {
			config = &tls.Config{}
		} else {
			config = config.Clone()
		}

		if err := ktls.Modifiers(mods).Apply(config); err != nil {
			return err
		}

		transport.TLSClientConfig = config
		return nil
	}
}

// WithCompression enables/disables compression (enabled by default).
func WithCompression2(enabled bool) Modifier2 {
	return func(transport *http2.Transport) error {
		transport.DisableCompression = !enabled
		return nil
	}
}

// WithReadIdleTimeout configures a ReadIdleTimeout as per http2.Transport docs.
func WithReadIdleTimeout2(value time.Duration) Modifier2 {
	return func(transport *http2.Transport) error {
		transport.ReadIdleTimeout = value
		return nil
	}
}

// WithPingTimeout configures a PingTimeout as per http2.Transport docs.
func WithPingTimeout2(value time.Duration) Modifier2 {
	return func(transport *http2.Transport) error {
		transport.PingTimeout = value
		return nil
	}
}

// WithWriteByteTimeout configures a WriteByteTimeout as per http2.Transport docs.
func WithWriteByteTimeout2(value time.Duration) Modifier2 {
	return func(transport *http2.Transport) error {
		transport.WriteByteTimeout = value
		return nil
	}
}

// WithHTTP2EnabledOptions configures an http2.Transport tied to an http.Transport.
//
// By default, an http.Transport in golang can use http2. This is implemented
// internally by using an http2.Transport when both the client and server
// support the protocol. When an http2.Transport is not configured explicitly,
// a default one is used.
//
// WithHTTP2EnabledOptions will enable http2, create a new http2.Transport
// object and associate it to the supplied http.Transport by invoking
// http2.ConfigureTransports.
//
// In short, it ensures that http2 is enabled with the specified options
// (rather than the default ones).
//
// IMPORTANT: it is an error to invoke WithHTTP2Options or WithHTTP2EnabledOptions
// multiple times. The golang API does not allow access to the http2 parameters
// after they have been bound to an http transport.
func WithHTTP2EnabledOptions(mods ...Modifier2) Modifier {
	return func(transport *http.Transport) error {
		h2, err := http2.ConfigureTransports(transport)
		if err != nil {
			return err
		}

		if err := Modifiers2(mods).Apply(h2); err != nil {
			return err
		}
		return nil
	}
}

// WithHTTP2Options applies the http2 options if http2 is enabled.
//
// Differently from WithHTTP2EnabledOptions, it will only apply the
// options if http2 is already enabled.
//
// IMPORTANT: it is an error to invoke WithHTTP2Options or WithHTTP2EnabledOptions
// multiple times. The golang API does not allow access to the http2 parameters
// after they have been bound to an http transport.
func WithHTTP2Options(mods ...Modifier2) Modifier {
	return func(transport *http.Transport) error {
		if len(mods) <= 0 || !HasHTTP2Enabled(transport) {
			return nil
		}

		return WithHTTP2EnabledOptions(mods...)(transport)
	}
}

// WithHTTP2Disabled disables the HTTP/2 upgrade and transport.
func WithHTTP2Disabled() Modifier {
	return func(transport *http.Transport) error {
		// Quoting godoc for net/http:
		// 	Starting with Go 1.6, the http package has transparent
		// 	support for the HTTP/2 protocol when using HTTPS.
		// 	Programs that must disable HTTP/2 can do so by setting
		// 	Transport.TLSNextProto (for clients) or
		// 	Server.TLSNextProto (for servers) to a non-nil, empty
		// 	map.
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		return nil
	}
}

// HasHTTP2Enabled returns true if http2 is enabled on a transport.
func HasHTTP2Enabled(t *http.Transport) bool {
	if t.TLSNextProto == nil {
		return true
	}

	_, found := t.TLSNextProto["h2"]
	return found
}
