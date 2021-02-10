package protocol

import (
	"context"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type Cleaner func()

type Cleaners []Cleaner

func (c Cleaners) Run() {
	for _, cleaner := range c {
		cleaner()
	}
}

type Options struct {
	Url    string
	Method string

	Handler ResponseHandler
	Cleaner []Cleaner

	Client  *http.Client
	Ctx     context.Context
	Timeout time.Duration
	Content io.Reader

	ClientMods  kclient.Modifiers
	RequestMods krequest.Modifiers
}

type Modifier func(o *Options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(o *Options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func WithRequestOptions(mods ...krequest.Modifier) Modifier {
	return func(o *Options) error {
		o.RequestMods = append(o.RequestMods, mods...)
		return nil
	}
}

func WithClientOptions(mods ...kclient.Modifier) Modifier {
	return func(o *Options) error {
		o.ClientMods = append(o.ClientMods, mods...)
		return nil
	}
}

func WithContext(ctx context.Context) Modifier {
	return func(o *Options) error {
		o.Ctx = ctx
		return nil
	}
}

func WithTimeout(timeout time.Duration) Modifier {
	return func(o *Options) error {
		o.Timeout = timeout
		return nil
	}
}

// WithContent allows to provide a body to be sent to the remote end.
//
// For example, use WithContent(strings.NewReader("this is some json")) to
// send some json alongside your POST requests.
func WithContent(content io.Reader) Modifier {
	return func(o *Options) error {
		o.Content = content
		return nil
	}
}

func WithOptions(mods ...Modifier) Modifier {
	return func(o *Options) error {
		return Modifiers(mods).Apply(o)
	}
}

func WithCleaner(cleaner Cleaner) Modifier {
	return func(o *Options) error {
		o.Cleaner = append(o.Cleaner, cleaner)
		return nil
	}
}

// Get will perform a GET request to retrieve the specified url by invoking the Do method.
func Get(url string, handler ResponseHandler, mod ...Modifier) error {
	return Do(http.MethodGet, url, handler, mod...)
}

// Get will perform a POST request to retrieve the specified url by invoking the Do method.
func Post(url string, handler ResponseHandler, mod ...Modifier) error {
	return Do(http.MethodPost, url, handler, mod...)
}

// Do performs an http request by applying the specified options.
//
// method is a string, generally one of the http.Method.* constants, indicating
// the HTTP method to use.
// url is the remote url to retrieve, as a string.
// mod is a set of modifiers indicating how to perform the request.
func Do(method, url string, handler ResponseHandler, mod ...Modifier) error {
	options := &Options{
		Url:     url,
		Method:  method,
		Handler: handler,
		Timeout: 30 * time.Second,
		Client:  &http.Client{},
	}

	err := Modifiers(mod).Apply(options)
	defer Cleaners(options.Cleaner).Run()
	if err != nil {
		return err
	}
	return options.Do()
}

func (options *Options) Do() error {
	url := options.Url
	ctx := options.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	if err := options.ClientMods.Apply(options.Client); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, options.Method, url, options.Content)
	if err != nil {
		return err
	}

	var resp *http.Response
	if err = options.RequestMods.Apply(req); err == nil {
		req.URL.RawQuery = req.URL.Query().Encode()

		// dump, err := httputil.DumpRequest(req, true)
		// log.Printf("REQUEST %v - %s", err, dump)

		resp, err = options.Client.Do(req)

		// dump, err = httputil.DumpResponse(resp, true)
		// log.Printf("RESPONSE %v - %s", err, dump)
	}

	defer func() {
		if resp == nil {
			return
		}

		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	return options.Handler(url, resp, err)
}
