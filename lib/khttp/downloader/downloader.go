package downloader

import (
	"context"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/scheduler"
	"github.com/enfabrica/enkit/lib/khttp/workpool"
	"github.com/enfabrica/enkit/lib/retry"
	"sync"
	"time"
)

type roptions struct {
	ctx context.Context

	protocol protocol.Modifiers
	client   kclient.Modifiers
	request  krequest.Modifiers

	retry   retry.Modifiers
	timeout time.Duration
}

type Flags struct {
	Timeout  time.Duration
	Retry    *retry.Flags
	Workpool *workpool.Flags
	Client   *kclient.Flags
}

func DefaultFlags() *Flags {
	flags := &Flags{
		Timeout:  time.Second * 5,
		Retry:    retry.DefaultFlags(),
		Workpool: workpool.DefaultFlags(),
		Client:   kclient.DefaultFlags(),
	}
	return flags
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.DurationVar(&fl.Timeout, prefix+"download-timeout", fl.Timeout, "Overall timeout when attempting download operations")

	fl.Retry.Register(set, prefix+"download-")
	fl.Workpool.Register(set, prefix+"download-")
	fl.Client.Register(set, prefix+"download-")
	return fl
}

func FromFlags(fl *Flags) Modifier {
	return func(o *options) error {
		if fl == nil {
			return nil
		}

		o.timeout = fl.Timeout
		o.retry = append(o.retry, retry.FromFlags(fl.Retry))
		o.pool = append(o.pool, workpool.FromFlags(fl.Workpool))
		o.client = append(o.client, kclient.FromFlags(fl.Client))
		return nil
	}
}

func WithTimeout(timeout time.Duration) Modifier {
	return func(o *options) error {
		o.timeout = timeout
		return nil
	}
}

func WithRetryOptions(mods ...retry.Modifier) Modifier {
	return func(o *options) error {
		o.retry = append(o.retry, mods...)
		return nil
	}
}

func WithProtocolOptions(mods ...protocol.Modifier) Modifier {
	return func(o *options) error {
		o.protocol = append(o.protocol, mods...)
		return nil
	}
}

func WithRequestOptions(mods ...krequest.Modifier) Modifier {
	return func(o *options) error {
		o.request = append(o.request, mods...)
		return nil
	}
}

func WithClientOptions(mods ...kclient.Modifier) Modifier {
	return func(o *options) error {
		o.client = append(o.client, mods...)
		return nil
	}
}

func WithWaitGroup(wg *sync.WaitGroup) Modifier {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

func WithWorkpoolOptions(mods ...workpool.Modifier) Modifier {
	return func(o *options) error {
		o.pool = append(o.pool, mods...)
		return nil
	}
}

func WithSchedulerOptions(mods ...scheduler.Modifier) Modifier {
	return func(o *options) error {
		o.sched = append(o.sched, mods...)
		return nil
	}
}

type options struct {
	roptions

	sched scheduler.Modifiers
	pool  workpool.Modifiers
	wg    *sync.WaitGroup
}

type Downloader struct {
	roptions

	wg    *sync.WaitGroup
	sched *scheduler.Scheduler
	pool  *workpool.WorkPool
}

// Retrier returns a retrier with the same options that would be used by the downloader.
func (o *roptions) Retrier() *retry.Options {
	return retry.New(o.retry...)
}

func (o *roptions) ProtocolModifiers() []protocol.Modifier {
	return append(protocol.Modifiers{
			protocol.WithContext(o.ctx),
			protocol.WithTimeout(o.timeout),
			protocol.WithRequestOptions(o.request...),
			protocol.WithClientOptions(o.client...)}, o.protocol...)
}

// Get will fetch the specified url, invoke handler to process the response, and eh to process the returned error.
//
// Get will schedule the operation through a workpool.
//
// The operation will fail if something goes wrong with the HTTP request, or if the handler returns error.
// Regardless, the error handler is invoked with the result of the Get (which could be nil, to indicate success).
//
// When combining with WithRetry options, if the handler or get return error, the operation will be retried.
//
// Get returns an error. But given that downloads are scheduled asynchronously, the only case when Get will
// return an error is if an invalid combination of flags or options was specified.
func (d *Downloader) Get(url string, handler protocol.ResponseHandler, eh workpool.ErrorHandler, mods ...Modifier) error {
	options := &options{roptions: d.roptions}
	if err := Modifiers(mods).Apply(options); err != nil {
		return err
	}

	work := func() error {
		return protocol.Get(url, handler, options.ProtocolModifiers()...)
	}

	retrier := options.Retrier()
	if retrier.AtMost <= 0 {
		d.pool.Add(workpool.WithError(work, eh))
	} else {
		d.pool.Add(workpool.WithRetry(retrier, d.sched, d.pool, work, eh))
	}
	return nil
}

type Modifier func(*options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(o *options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func WithContext(ctx context.Context) Modifier {
	return func(o *options) error {
		o.ctx = ctx
		return nil
	}
}

func (d *Downloader) Wait() {
	d.wg.Wait()
}

func New(mods ...Modifier) (*Downloader, error) {
	options := &options{
		roptions: roptions{
			ctx: context.Background(),
		},
		wg: &sync.WaitGroup{},
	}
	if err := Modifiers(mods).Apply(options); err != nil {
		return nil, err
	}

	wp, err := workpool.New(append([]workpool.Modifier{
		workpool.WithWaitGroup(options.wg)}, options.pool...)...)
	if err != nil {
		return nil, err
	}

	return &Downloader{
		roptions: options.roptions,
		wg:       options.wg,
		sched: scheduler.New(append([]scheduler.Modifier{
			scheduler.WithWaitGroup(options.wg)}, options.sched...)...),
		pool: wp,
	}, nil
}
