package multierror

import (
	"errors"
	"strings"
)

const Separator = "\n  "

type MultiError []error

// New creates a MultiError from a list of errors.
// It has two primary intended uses:
// * Capture maybe returns of ACID-style transactions or stack tracing.
// 		> myTypedErr := errors.New("my specific query failed in transaction")
//		> return multierror.Wrap(myTypedErr, tx.Rollback())
//		> errors.Is(err, &myTypedErr)
//
//
// if errs has len() == 0, or all errors are nil, nil is returned.
// * Capture tracing from fmt.Errorf
// 		> myTypedErr := errors.New("ssh agent failed to query keyring")
//		> return multierror.Wrap(myTypedErr, fmt.Errorf("mw raw log %v", realAgentErr))
//		> if errors.Is(err, &myTypedErr); fmt.Println("I know it was the keyring, here is the stack ", err.Error())
//
func New(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	for _, err := range errs {
		if err != nil {
			return MultiError(errs)
		}
	}
	return nil
}

func Wrap(err ...error) error {
	return New(err)
}

// NewOr creates a MultiError from a list of errors, or returns the fallback error.
//
// empty, it will return the supplied error instead.
func NewOr(errs []error, fallback error) error {
	if len(errs) == 0 {
		return fallback
	}
	for _, err := range errs {
		if err != nil {
			return MultiError(errs)
		}
	}
	return fallback
}

// Unwrap for MultiError always returns nil, as there is no reasonable way to implement it.
//
// Use As and Is methods, or loop over the list directly to access the underlying errors.
func (multi MultiError) Unwrap() error {
	if len(multi) == 0 {
		return nil
	}
	return multi[1:]
}

// As for MultiError returns the first error that can be considered As the specified target.
func (multi MultiError) As(target interface{}) bool {
	if len(multi) == 0 {
		return false
	}
	return errors.As(multi[0], target)
}

// Is for MultiError returns true if any of the errors listed can be considered of the target type.
func (multi MultiError) Is(target error) bool {
	if len(multi) == 0 {
		return false
	}
	return errors.Is(multi[0], target)
}

func (multi MultiError) Error() string {
	var messages []string
	for _, err := range multi {
		if err == nil {
			continue
		}
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, Separator)
}
