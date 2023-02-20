// A simple library to safely retry operations.
//
// Whenever you have an operation that can temporarily fail, your code
// should have a strategy to retry the operation.
//
// But retrying is non trivial: before retrying, your code should wait
// some time. It should also not retry forever, and eventually give up.
// It should allow you to handle fatal errors and temporary errors
// differently. If the request blocks for a long time before failing,
// it should probably take into account that time when deciding how
// long to wait before retrying.
//
// If you are writing an application that is talking with a remote
// endpoint and will be running on a large number of machines, your code should
// also try to randomize the retry interval over a period of time, so that if
// a remote endpoint experiences an outage, and all clients try to reconnect,
// they don't all reconnect at the same time. This is important to prevent
// the "thundering herd" problem, which could overload the remote backend,
// further prolonging the outage.
//
// To use the retry library:
//
// 1) Create a `retry.Options` object, like:
//
//	options := retry.New(retry.WithWait(5 * time.Second), retry.WithAttempts(10))
//
// 2) Run some code:
//
//	options.Run(func () error {
//	  ...
//	})
//
// The retry library will run your functions as many times as configured until it
// returns an error, or until it returns retry.FatalError (use retry.Fatal to
// create one) or an error wrapping a retry.FatalError (see the errors library,
// and all the magic wrapping/unwrapping logic).
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
//	attempt #1 - FAILED - This is the string error received
//
// If you provide a description instead, you will get a log entry:
//
//	attempt #1 Your description goes here - FAILED - This is the string error received
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

// WithLogger configures a logger to send log messages to.
func WithLogger(log logger.Logger) Modifier {
	return func(o *Options) {
		o.logger = log
	}
}

// WithTimeSource configures a different clock.
func WithTimeSource(ts TimeSource) Modifier {
	return func(o *Options) {
		o.Now = ts
	}
}

// FromFlags configures a retry object from command line flags.
func FromFlags(fl *Flags) Modifier {
	return func(o *Options) {
		if fl == nil {
			return
		}

		o.Flags = *fl
	}
}

// New creates a new retry object.
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

func (s *FatalError) Unwrap() error {
	return s.Original
}

// Fatal turns a normal error into a fatal error.
//
// Fatal errors will stop the retrier immediately.
// Fatal errors implement the Unwrap() API, allowing the use of
// errors.Is, errors.As, and errors.Unwrap.
func Fatal(err error) *FatalError {
	return &FatalError{Original: err}
}

// DelaySince computes how longer to wait since a start time.
//
// DelaySince assumes that a wait started at start time, and computes
// how longer the code still has to wait based on a delay computed
// with the Delay() function.
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

// Delay computes how long to wait before the next attempt.
//
// If Fuzzy is non 0, the delay is fuzzied by a random amount
// less than the value of fuzzy.
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

// ExaustedError is returned when the retrier has exhausted all attempts.
type ExaustedError struct {
	// Message is a human readable error message, returned by Error().
	Message string
	// Original is a multierror.MultiError containing the last MaxErrors errors.
	Original error
}

func (ee *ExaustedError) Error() string {
	return ee.Message
}

func (ee *ExaustedError) Unwrap() error {
	return ee.Original
}

// Once runs the specified function once as if it was run by Run().
//
// attempt is the attempt number, how many times before it was invoked.
// runner is the function to invoke.
//
// Once returns the error returned by the supplied runner.
// In case the runner fails, Once also log messages as specified by Options
// and exactly like Run() would, and computes a delay indicating how long to
// wait before running this function again.
//
// Once is useful in non-blocking or multithreaded code, when you cannot
// afford to block an entire goroutine for the funcntion to complete, but
// you still want to implement reasonable retry semantics based on this
// library.
//
// Typically, your code will invoke Once() to run the runner, and in case of
// failure, re-schedule it to run later.
func (o *Options) Once(attempt int, runner func() error) (time.Duration, error) {
	return o.OnceAttempt(attempt, func(attempt int) error {
		return runner()
	})
}

// OnceAttempt is just like Once, but invokes a runner that expects an attempt #.
//
// OnceAttempt is to Once what RunAttempt is to Run. Read the documentation for
// RunAttempt and Once for details.
func (o *Options) OnceAttempt(attempt int, runner func(attempt int) error) (time.Duration, error) {
	description := ""
	if o.description != "" {
		description = " - " + o.description
	}

	start := o.Now()
	err := runner(attempt)
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

// Run runs the function specified until it succeeds.
//
// Run will keep retrying running the function until either the function
// return a nil error, it returns a FatalError, or all retry attempts as
// specified in Options are exhausted.
//
// When Run gives up running a function, it returns the original error
// returned by the function, wrapped into an ExaustedError.
//
// You can use errors.As or errors.Is or the unwrap magic to retrieve
// the original error.
func (o *Options) Run(runner func() error) error {
	return o.RunAttempt(func(attempt int) error {
		return runner()
	})
}

// RunAttempt is just like Run, but propagates the attempt #.
//
// Use RunAttempt when your function callback requires knowing how
// many attemps have been made so far at running your function.
// This is useful, for example, to log an extra message every x
// attempts, to re-initialize state on non-first attempt, or
// try harder after a number of attempts, ...
func (o *Options) RunAttempt(runner func(attempt int) error) error {
	errs := []error{}
	for ix := 0; o.AtMost == 0 || ix < o.AtMost; ix++ {
		delay, err := o.OnceAttempt(ix, runner)
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
