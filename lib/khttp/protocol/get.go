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

func Get(url string, handler ResponseHandler, mod ...Modifier) error {
	options := &Options{
		Url:     url,
		Method:  http.MethodGet,
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

	req, err := http.NewRequestWithContext(ctx, options.Method, url, nil)
	if err != nil {
		return err
	}

	var resp *http.Response
	if err = options.RequestMods.Apply(req); err == nil {
		resp, err = options.Client.Do(req)
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
