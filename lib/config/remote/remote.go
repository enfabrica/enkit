// A config Store able to load and store configs via simple HTTP requests.
package remote

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/mitchellh/mapstructure"
)

type DNSFlags struct {
	Timeout time.Duration
	Prefix  string
	Retry   *retry.Flags
}

func DefaultDNSFlags() *DNSFlags {
	return &DNSFlags{
		Timeout: 3 * time.Second,
		Prefix:  "_enkit_config_v2",
		Retry:   retry.DefaultFlags(),
	}
}

func (fl *DNSFlags) Register(set kflags.FlagSet, prefix string) *DNSFlags {
	set.DurationVar(&fl.Timeout, prefix+"dns-timeout", fl.Timeout, "How long to wait for a DNS response before giving up")
	set.StringVar(&fl.Prefix, prefix+"dns-record", fl.Prefix, "Which DNS record to look up to find the relevant TXT fields")
	fl.Retry.Register(set, prefix+"dns-")
	return fl
}

type DNS struct {
	DNSFlags

	domain string

	log   logger.Logger
	retry *retry.Options
}

type DNSOptions map[string]string

func (do DNSOptions) Apply(target interface{}, fns ...mapstructure.DecodeHookFunc) ([]string, error) {
	md := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			append([]mapstructure.DecodeHookFunc{
				mapstructure.StringToTimeHookFunc(time.RFC3339Nano),
				mapstructure.StringToTimeDurationHookFunc()}, fns...)...),

		Result:           target,
		WeaklyTypedInput: true,
		Metadata:         &md,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	return md.Unused, decoder.Decode(do)
}

func ParseDNSOptions(value string) (DNSOptions, error) {
	fields := strings.Fields(value)

	var opts DNSOptions
	for _, field := range fields {
		option, err := url.PathUnescape(field)
		if err != nil {
			return nil, fmt.Errorf("invalid url encoding? could not unescape %s - %w", field, err)
		}

		equal := strings.Index(option, "=")
		if equal < 0 {
			return nil, fmt.Errorf("invalid option %s in %s - does not have equal", option, fields)
		}

		key := strings.TrimSpace(option[:equal])
		if len(key) <= 0 {
			return nil, fmt.Errorf("invalid empty key - an = sign with a space beforehand? %s", fields)
		}
		value := strings.TrimSpace(option[equal+1:])

		_, found := opts[key]
		if found {
			return nil, fmt.Errorf("option %s was already set in %s", option, fields)
		}
		if opts == nil {
			opts = DNSOptions{}
		}
		opts[key] = value
	}

	return opts, nil
}

// ParseTXTRecord decodes the content of a TXT record.
//
// TXT records are expected to either be a simple URL, like "http://mydomain.com/configs/",
// or a set of options followed by |, followed by the URL.
//
// For example:
//   timeout=3 retries=5|http://mydomain.com/configs/
//
// If the option needs to contain the | or any other forbidden character, the option can
// be URL path encoded, with characters replaced by % notation. % itself needs to be escaped.
func ParseTXTRecord(record string) (DNSOptions, *url.URL, error) {
	address := record
	options := ""
	ix := strings.Index(record, "|")
	if ix >= 0 {
		options = record[:ix]
		address = record[ix+1:]
	}

	u, err := url.Parse(address)
	if err != nil {
		return nil, nil, fmt.Errorf("record %s contains invalid URL - %w", record, err)
	}

	parsed, err := ParseDNSOptions(options)
	if err != nil {
		return nil, nil, fmt.Errorf("record %s contains invalid options - %w", record, err)
	}

	return parsed, u, nil
}

type DNSModifier func(*DNS)

type DNSModifiers []DNSModifier

func WithLogger(log logger.Logger) DNSModifier {
	return func(d *DNS) {
		d.log = log
	}
}

func WithTimeout(duration time.Duration) DNSModifier {
	return func(d *DNS) {
		d.Timeout = duration
	}
}

func WithPrefix(prefix string) DNSModifier {
	return func(d *DNS) {
		d.Prefix = prefix
	}
}

func WithRetry(retry *retry.Options) DNSModifier {
	return func(d *DNS) {
		d.retry = retry
	}
}

func FromDNSFlags(flags *DNSFlags) DNSModifier {
	return func(d *DNS) {
		if flags == nil {
			return
		}

		d.DNSFlags = *flags
	}
}

func NewDNS(domain string, mods ...DNSModifier) *DNS {
	retval := &DNS{
		DNSFlags: *DefaultDNSFlags(),
		domain:   domain,
		log:      logger.Nil,
	}

	for _, m := range mods {
		m(retval)
	}

	if retval.retry == nil {
		retval.retry = retry.New(retry.WithLogger(retval.log), retry.FromFlags(retval.Retry))
	}
	return retval
}

func (d *DNS) Name() string {
	return d.Prefix + "." + d.domain
}

type NotFoundError struct {
	Record string
}

func (e *NotFoundError) Error() string {
	return "TXT record for " + e.Record + " not found"
}

func (d *DNS) GetEndpoints() ([]Endpoint, []error) {
	var records []string
	err := d.retry.Run(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), d.Timeout)
		defer cancel()

		var err error
		records, err = net.DefaultResolver.LookupTXT(ctx, d.Name())
		if derr, ok := err.(*net.DNSError); ok && derr.IsNotFound {
			return retry.Fatal(err)
		}
		return err
	})
	if err != nil {
		return nil, []error{fmt.Errorf("could not resolve %s - %w", d.Name(), err)}
	}

	errs := []error{}
	endpoints := []Endpoint{}
	for _, record := range records {
		options, url, err := ParseTXTRecord(record)
		if err != nil {
			errs = append(errs, fmt.Errorf("in %s TXT record - %w", d.Name(), err))
			continue
		}
		endpoints = append(endpoints, Endpoint{
			Options: options,
			URL:     url,
		})
	}
	return endpoints, errs
}

func (d *DNS) Open(app string, namespaces ...string) (*Remote, error) {
	endpoints, errs := d.GetEndpoints()

	if len(endpoints) > 0 {
		http, err := NewHTTP(endpoints)
		if err != nil {
			return nil, multierror.New(append(errs, err))
		}
		for _, e := range errs {
			d.log.Infof("endpoint ignored due to error: %s", e)
		}
		return http.Open(app, namespaces...)
	}

	return nil, multierror.NewOr(errs, &NotFoundError{Record: d.Name()})
}

type Endpoint struct {
	Options DNSOptions
	URL     *url.URL
	Timeout time.Duration
}

type WritePolicy int

const (
	// Attempts to write to all endpoints. Succeeds if all succeed.
	WriteSucceedAll WritePolicy = iota
	// Attempts to write to all endpoints. Succeeds if at least one write succeeds.
	WriteSucceedOne
	// Attempts to write until a write succeeds. Not all endpoints will be written to.
	WriteFirst
)

type HTTP struct {
	endpoints []Endpoint
	policy    WritePolicy
}

func NewHTTP(endpoints []Endpoint) (*HTTP, error) {
	if len(endpoints) <= 0 {
		return nil, fmt.Errorf("at least one endpoint must be specified")
	}

	for _, ep := range endpoints {
		if ep.URL.Scheme != "http" && ep.URL.Scheme != "https" {
			return nil, fmt.Errorf("url %s uses invalid scheme - must use either http or https", ep.URL)
		}
	}

	return &HTTP{endpoints: endpoints}, nil
}

type Remote struct {
	http *HTTP
	path string
}

func (u *HTTP) Open(app string, namespaces ...string) (*Remote, error) {
	return &Remote{http: u, path: path.Join(append([]string{app}, namespaces...)...)}, nil
}

func WriteEndpoint(endpoint Endpoint, upath string, data []byte) error {
	client := &http.Client{}

	timeout := endpoint.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Make a copy of the url.
	url := *endpoint.URL
	url.Path = path.Join(url.Path, upath)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request returned status code %d - %s", resp.StatusCode, resp.Status)
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func ReadEndpoint(endpoint Endpoint, upath string) ([]byte, error) {
	client := &http.Client{}

	timeout := endpoint.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Make a copy of the url.
	url := *endpoint.URL
	url.Path = path.Join(url.Path, upath)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request returned status code %d - %s", resp.StatusCode, resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Remote) Read(name string) ([]byte, error) {
	errs := []error{}
	for _, endpoint := range r.http.endpoints {
		data, err := ReadEndpoint(endpoint, path.Join(r.path, name))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return data, nil
	}
	return nil, multierror.New(errs)
}

func (r *Remote) Write(name string, data []byte) error {
	errs := []error{}
	for _, endpoint := range r.http.endpoints {
		err := WriteEndpoint(endpoint, path.Join(r.path, name), data)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if r.http.policy == WriteFirst {
			return nil
		}
	}

	if len(errs) < len(r.http.endpoints) && r.http.policy == WriteSucceedOne {
		return nil
	}
	return multierror.New(errs)
}
