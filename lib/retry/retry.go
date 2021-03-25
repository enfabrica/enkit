package retry

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
)

// TimeSource is a function returning the current time. Generally, it should be set to time.Now.
// Mainly used for testing.
type TimeSource func() time.Time

// Options are all the options that the Retry functions accept.
type Options struct {
	rng *rand.Rand
	// Log retries using this logger.
	logger logger.Logger
	// Description to add to log messages.
	description string

	// How to read time.
	Now TimeSource

	Flags
}

// Nil is a set of retry options that perform a single retry.
//
// This is useful whenever you have an object that requires a retry config, but you only
// want a single retry attempt to be performed.
var Nil = &Options{
	logger: logger.Nil,
	Now:    time.Now,
	Flags: Flags{
		AtMost: 1,
	},
}

type Flags struct {
	// How many times to retry the operation, at most.
	AtMost int
	// How long to wait between attempts.
	Wait time.Duration
	// How much of a random retry time to add.
	Fuzzy time.Duration
	// How many errors to store at most.
	MaxErrors int
}

func DefaultFlags() *Flags {
	return &Flags{
		AtMost:    5,
		Wait:      1 * time.Second,
		Fuzzy:     1 * time.Second,
		MaxErrors: 10,
	}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.IntVar(&fl.AtMost, prefix+"retry-at-most", fl.AtMost, "How many time to retry the operation at most")
	set.IntVar(&fl.MaxErrors, prefix+"retry-max-errors", fl.MaxErrors, "How many errors to record when retrying")
	set.DurationVar(&fl.Wait, prefix+"retry-wait", fl.Wait, "How long to wait from the start of an attempt to the next")
	set.DurationVar(&fl.Fuzzy, prefix+"retry-fuzzy", fl.Fuzzy, "How much randomized time to add to each retry-wait time")
	return fl
}

type Modifier func(*Options)

type Modifiers []Modifier

func (mods Modifiers) Apply(o *Options) *Options {
	for _, m := range mods {
		m(o)
	}
	return o
}

// WithDescription adds text used from logging, to distinguish a retry attempt from another.
//
// If no description is provided, and retry fails, you will get a generic log entry like:
//
//     attempt #1 - FAILED - This is the string error received
//
// If you provide a description instead, you will get a log entry:
//
//     attempt #1 Your description goes here - FAILED - This is the string error received
//
func WithDescription(desc string) Modifier {
	return func(o *Options) {
		o.description = desc
	}
}

// WithRng sets a random number generator to use. If not set, it just uses math.Rand.
// Convenient for testing, or to set a seeded / secure global generator.
func WithRng(rng *rand.Rand) Modifier {
	return func(o *Options) {
		o.rng = rng
	}
}

// WithWait sets how long to wait between attempts.
//
// Note that retry will start counting the time since the last attempt was started.
//
// Let's say you use retry to connect to a remote server. You set the Wait time to
// 10 seconds. The connection succeeds at 2pm. At 3pm, one hour later, the connection
// fails, and retry kicks in. Retry will retry *immediately* as 10 seconds passed
// already since the last connection attempt.
//
// The server is now down, and connecting fails in 5 seconds. Retry will wait 5 more
// seconds to reconnect.
//
// In general, make sure that your Wait time is set > than the timeout configured
// for whatever operation is attempted, otherwise it will almost always reconnect
// immediately.
//
// Another way to look at it: the Wait time guarantees that there is no more than
// one attempt at the operation within the Wait time.
func WithWait(duration time.Duration) Modifier {
	return func(o *Options) {
		o.Wait = duration
	}
}

// WithFuzzy introduces a random offset from 0 to fuzzy time in between connection attempts.
//
// This is very important in distributed environments, to avoid connection storms or
// overload because of a failure.
//
// For example: let's say that you have 10,000 workers, connected to a server. The
// server crashes at 2pm. With no fuzzy time, all the 10,000 workers will likely try
// to reconnect at exactly the same time.
//
// If you set fuzzy time to 10 seconds, a random retry time up to 10 seconds will
// be added to the normal retry time.
//
// This will cause the server to process roughly 1,000 requests per second,
// rather than 10,000.
func WithFuzzy(fuzzy time.Duration) Modifier {
	return func(o *Options) {
		o.Fuzzy = fuzzy
	}
}

// WithAttempts configures the number of attempts to perform.
func WithAttempts(atmost int) Modifier {
	return func(o *Options) {
		o.AtMost = atmost
	}
}

func WithLogger(log logger.Logger) Modifier {
	return func(o *Options) {
		o.logger = log
	}
}
func WithTimeSource(ts TimeSource) Modifier {
	return func(o *Options) {
		o.Now = ts
	}
}

func FromFlags(fl *Flags) Modifier {
	return func(o *Options) {
		if fl == nil {
			return
		}

		o.Flags = *fl
	}
}

func New(mods ...Modifier) *Options {
	return Modifiers(mods).Apply(&Options{
		Flags:  *DefaultFlags(),
		Now:    time.Now,
		logger: logger.Nil,
	})
}

type FatalError struct {
	Original error
}

func (s *FatalError) Error() string {
	if s.Original != nil {
		return s.Original.Error()
	}
	return "requested to stop retrying"
}

func Fatal(err error) *FatalError {
	return &FatalError{Original: err}
}

func (o *Options) DelaySince(start time.Time) time.Duration {
	delay := o.Delay()
	if start.IsZero() {
		return delay
	}

	elapsed := time.Now().Sub(start)
	if elapsed >= delay {
		return 0
	}
	return delay - elapsed
}

func (o *Options) Delay() time.Duration {
	delta := int64(0)
	if o.Fuzzy > 0 {
		r := rand.Int63n
		if o.rng != nil {
			r = o.rng.Int63n
		}
		delta = r(int64(o.Fuzzy))
	}
	return o.Wait + time.Duration(delta)
}

type ExaustedError struct {
	Message  string
	Original error
}

func (ee *ExaustedError) Error() string {
	return ee.Message
}

func (ee *ExaustedError) Unwrap() error {
	return ee.Original
}

func (o *Options) Once(attempt int, runner func() error) (time.Duration, error) {
	description := ""
	if o.description != "" {
		description = " - " + o.description
	}

	start := o.Now()
	err := runner()
	if err == nil {
		return 0, nil
	}

	var stop *FatalError
	var delay time.Duration
	message := "considered FATAL - not retrying anymore"
	format := "attempt #%d%s - FAILED - %s - %s"
	if errors.As(err, &stop) {
		o.logger.Errorf(format, attempt+1, description, err, message)
	} else {
		delay = o.DelaySince(start)
		if delay > 0 {
			message = fmt.Sprintf("will retry in %s", delay)
		} else {
			message = "retrying immediately"
		}
		o.logger.Infof(format, attempt+1, description, err, message)
	}
	return delay, err
}

func (o *Options) Run(runner func() error) error {
	errs := []error{}
	for ix := 0; o.AtMost == 0 || ix < o.AtMost; ix++ {
		delay, err := o.Once(ix, runner)
		if err == nil {
			return nil
		}
		var stop *FatalError
		if errors.As(err, &stop) {
			return stop.Original
		}

		if len(errs) <= o.MaxErrors {
			errs = append(errs, err)
		}

		if delay > 0 {
			time.Sleep(delay)
		}
	}
	err := multierror.New(errs)
	return &ExaustedError{Original: err, Message: fmt.Sprintf("gave up after %d attempts - %s", o.AtMost, err)}
}
