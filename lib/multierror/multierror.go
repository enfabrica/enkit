package multierror

import (
	"errors"
	"strings"
)

const Seperator = "\n "

type MultiError struct {
	err1 error
	err2 error
}

var (
	_ error                           = &MultiError{}
	_ interface{ Unwrap() error }     = &MultiError{}
	_ interface{ Is(err error) bool } = &MultiError{}
)

// New creates a MultiError from a list of errors.
// It has two primary intended uses:
// * Capture maybe returns of ACID-style transactions or stack tracing.
// 		> myTypedErr := errors.New("my specific query failed in transaction")
//		> return multierror.Wrap(myTypedErr, tx.Rollback())
//		> errors.Is(err, &myTypedErr)
//
//
// * Capture tracing from fmt.Errorf
// 		> myTypedErr := errors.New("ssh agent failed to query keyring")
//		> return multierror.Wrap(myTypedErr, fmt.Errorf("mw raw log %v", realAgentErr))
//		> if errors.Is(err, &myTypedErr); fmt.Println("I know it was the keyring, here is the stack ", err.Error())
//
func New(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	l := len(errs)
	if l >= 2 {
		return &MultiError{err1: errs[0], err2: New(errs[1:])}
	}
	if l == 1 {
		return &MultiError{err1: errs[0], err2: nil}
	}
	return nil
}

func Wrap(ers ...error) error {
	return New(ers)
}

// NewOr creates a MultiError from a list of errors, or returns the fallback error.
//
// Just like New, but gurantees that an error is returned. If the list of errors is
// empty, it will return the supplied error instead.
func NewOr(errs []error, fallback error) error {
	if len(errs) == 0 {
		return fallback
	}
	return New(errs)
}

// Unwrap for MultiError always returns nil, as there is no reasonable way to implement it.
//
// Use As and Is methods, or loop over the list directly to access the underlying errors.
func (multi *MultiError) Unwrap() error {
	return multi.err2
}

// As for MultiError returns the first error that can be considered As the specified target.
func (multi MultiError) As(target interface{}) bool {
	if errors.As(multi.err1, target) {
		return true
	}
	return errors.As(multi.err2, target)
}

// Is for MultiError returns true if any of the errors listed can be considered of the target type.
func (multi MultiError) Is(target error) bool {
	if errors.Is(multi.err1, target) {
		return true
	}
	return errors.Is(multi.err2, target)
}

// Error unwraps a MultError recursively
func (multi *MultiError) Error() string {
	var messages []string
	if multi.err1 != nil {
		messages = append(messages, multi.err1.Error())
	}
	if multi.err2 != nil {
		messages = append(messages, multi.err2.Error())
	}
	return strings.Join(messages, Seperator)
}
