package workpool

import (
	"github.com/enfabrica/enkit/lib/khttp/scheduler"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/retry"
	"sync"
)

// ErrorWork represents Work that can return an error.
//
// Functions returning an error cannot be passed directly to a workpool or a scheduler.
// Instead, you can wrap those functions in WithRetry() or WithError(), defined below.
type ErrorWork func() error

// WithRetry will retry running the specified ErrorWork until it either succeeds, or
// the number of attempts has been exceeded.
//
// In case of failure, the ErrorHandler specified is invoked.
//
// This function is fully asynchronous: rather than block the worker thread until the attempts
// have been exhausted and the timers expired, it uses a scheduler to retry later, while freeing
// the worker thread.
//
// sched indicates a scheduler to use.
// wp indicates a WorkPool to use to re-schedule subsequent attempts.
// eh indicates what to do if - even after all attempts - the function is still failing.
func WithRetry(retries *retry.Options, sched *scheduler.Scheduler, wp *WorkPool, work ErrorWork, eh ErrorHandler) Work {
	tryer := Work(nil)
	errs := []error{}

	tryer = func() {
		wait, err := retries.Once(len(errs), work)
		if err == nil {
			eh.Handle(nil)
			return
		}

		errs = append(errs, err)
		if stop, ok := err.(*retry.FatalError); ok || len(errs) >= retries.AtMost {
			if ok {
				eh.Handle(stop.Original)
			} else {
				eh.Handle(multierror.New(errs))
			}
			return
		}

		sched.AddAfter(wait, func() {
			wp.AddImmediate(tryer)
		})
	}

	return tryer
}

// InGoRoutine will spawn a seperate go routine to run the Work.
//
// This is an anti-pattern for WorkPool, as the whole point of WorkPool is to have a fixed
// pool of coroutines to complete the work. However, this function is convenient when dealing
// with events scheduled via the scheduler package, or when there are large chunks of work
// that benefit from being queued within a WorkPool.
func InGoRoutine(work Work) Work {
	return func() {
		go work()
	}
}

// ResultHandler is the interface used by WithResult to handle a result.
//
// WithResult will just call the Handle() function as soon as the result of the Work
// is available for consumption.
type ResultHandler interface {
	Handle(interface{})
}

// ResultWork represents a function that returns some value (eg, an error, a string, ...).
type ResultWork func() interface{}

// WithResult collects the return value of a function and makes it available through a ResultHandler.
//
// For example, by doing:
//   result := ResultRetriever()
//   workpool.Add(WithResult(func() interface{} { return "hello" }, result))
//
// To retrieve the returned value, you can then run:
//   value := result.Get().(string)
//
// Get() will block until the work is completed.
//
// As an argument for WithResult, you can use anything implementing the ResultHandler
// interface, ResultRetriever() and ResultCallback() being the main implementations.
//
// To use ResultRetriever with functions returning multiple values, just wrap
// the returned values in an object.
func WithResult(w ResultWork, h ResultHandler) Work {
	return func() {
		result := w()
		h.Handle(result)
	}
}

// Result maintains state about the result of work passed to WithResult.
//
// Create Result objects ResultRetriever.
type Result struct {
	l      sync.Mutex
	result interface{}
}

// Handle implements the ResultHandler interface, is invoked internally by WithResult to feed a value.
func (p *Result) Handle(result interface{}) {
	p.result = result
	p.l.Unlock()
}

// Get returns the value returned by the executed work. It will block until the Work has run.
func (p *Result) Get() interface{} {
	p.l.Lock()
	return p.result
}

// ResultRetriever creates a result object.
func ResultRetriever() *Result {
	r := &Result{}
	r.l.Lock()
	return r
}

// ResultCallback invokes the specified callback as soon as the result is ready.
//
// Note that the callback will block the worker until completion. If this is not desireable,
// make sure your callback just schedules a goroutine.
//
// Use it like:
//  ..., WithResult(work, ResultCallback(handler))
type ResultCallback func(interface{})

func (c ResultCallback) Handle(result interface{}) {
	c(result)
}

type ErrorHandler interface {
	Handle(error)
}

func WithError(w ErrorWork, h ErrorHandler) Work {
	return func() {
		result := w()
		h.Handle(result)
	}
}

type ErrorCallback func(error)

func (c ErrorCallback) Handle(err error) {
	if err != nil {
		c(err)
	}
}

// ErrorLog returns an ErrorHandler that will just log the error message, and ignore it.
func ErrorLog(log logger.Printer) ErrorCallback {
	return func(err error) {
		if err != nil {
			log("%s", err)
		}
	}
}

// ErrorStore returns an ErrorHandler that will store the returned error into the specified pointer.
func ErrorStore(dest *error) ErrorCallback {
	return func(err error) {
		*dest = err
	}
}

// ErrorIgnore is an ErrorHandler that will ignore all errors.
var ErrorIgnore = ErrorCallback(func(error) {})

// ErrorResult returns an ErrorHandler that allows to retrieve the error returned by the work function.
//
// To use it:
//   delayedError := ErrorRetriever()
//   wp.Add(WithError(error, delayedError))
//   ...
//   err := delayedError.Get()
//
// Note that the Get function will block until the error has been returned.
func ErrorRetriever() *ErrorResult {
	return &ErrorResult{
		Result: ResultRetriever(),
	}
}

type ErrorResult struct {
	*Result
}

func (ep *ErrorResult) Get() error {
	return ep.Result.Get().(error)
}
func (ep *ErrorResult) Handle(result error) {
	ep.Result.Handle(result)
}
