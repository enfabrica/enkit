package multierror

import (
	"errors"
	"strings"
)

type MultiError []error

// New creates a MultiError from a list of errors.
//
// This is just a convenience wrapper to ensure that if there is no error,
// if errs has len() == 0, error is nil.
//
// In facts, you could normally convert a []error{} into a MultiError by simply
// using a cast: MultiError(errs). However, a simple cast would lead to a non
// nil error when errs has lenght 0.
func New(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return MultiError(errs)
}

// NewOr creates a MultiError from a list of errors, or returns the fallback error.
//
// Just like New, but gurantees that an error is returned. If the list of errors is
// empty, it will return the supplied error instead.
func NewOr(errs []error, fallback error) error {
	if len(errs) == 0 {
		return fallback
	}
	return MultiError(errs)
}

// Unwrap for MultiError always returns nil, as there is no reasonable way to implement it.
//
// Use As and Is methods, or loop over the list directly to access the underlying errors.
func (me MultiError) Unwrap() error {
	return nil
}

// As for MultiError returns the first error that can be considered As the specified target.
func (me MultiError) As(target interface{}) bool {
	for _, err := range me {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// Is for MultiError returns true if any of the errors listed can be considered of the target type.
func (me MultiError) Is(target error) bool {
	for _, err := range me {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (me MultiError) Error() string {
	if len(me) == 1 {
		return me[0].Error()
	}

	messages := []string{}
	for _, err := range me {
		messages = append(messages, err.Error())
	}
	return "Multiple errors:\n  " + strings.Join(messages, "\n  ")
}
