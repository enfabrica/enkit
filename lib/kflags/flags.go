package kflags

import (
	"fmt"
	"time"
)

// FlagSet interface provides an abstraction over a cobra or golang flag set, adding features from this library.
//
// Functions using this FlagSet interface can accept both a flag.FlagSet (from the standard golang library) and
// a pflag.FlagSet (from the spf13 pflag library), using the wrappers provided here and kcobra.
//
// Use this interface only when you are writing code that a) does not need any fancy pflag feature - and could
// work either way with flag or pflag - and/or b) relies on features of this library.
type FlagSet interface {
	BoolVar(p *bool, name string, value bool, usage string)
	DurationVar(p *time.Duration, name string, value time.Duration, usage string)
	StringVar(p *string, name string, value string, usage string)
	StringArrayVar(p *[]string, name string, value []string, usage string)
	ByteFileVar(p *[]byte, name string, defaultFile string, usage string, mods ...ByteFileModifier)
	IntVar(p *int, name string, value int, usage string)
}

// All flags have an associated Value: a boolean, a string, an integer, ...
//
// This is implemented by creating an object satisfying the flag.Value or pflag.Value interface, capable
// of converting a string passed via flags (--int-value="31") into the actualy target type.
//
// Normally, the string passed on the command line is directly converted into the target type. Eg, in
// the example above, the string "31" is converted into the integer 31, and stored into the pointer
// supplied via IntVar.
//
// This library defines flags that do not convert directly into the target type. For example, the
// ByteFile type creates a parameter that contains the path of a file name, but actually stores the
// byte content of the file.
//
// Any flag that provides a level of indirection should implement the ContentValue interface. In this
// case:
//   - the normal Set(string) would be used to set the value of the flag, which represents the
//     pointer, where to read the real value.
//   - the new SetContent(string) would be used to set the content of the flag, the value that the
//     flag would store in the target destination.
type ContentValue interface {
	SetContent(origin string, content []byte) error
}

// Consumer is any object that can take flags, and provides a common method
// to register flags.
type Consumer interface {
	Register(fs FlagSet, prefix string)
}

// Wrap errors in a StatusError to indicate a different exit value to be
// returned if the error causes the program to exit.
type StatusError struct {
	error
	Code int
}

func (se *StatusError) Unwrap() error {
	return se.error
}

func NewStatusError(code int, err error) *StatusError {
	return &StatusError{error: err, Code: code}
}

func NewStatusErrorf(code int, f string, args ...interface{}) *StatusError {
	return &StatusError{error: fmt.Errorf(f, args...), Code: code}
}

// Wrap errors in an UsageError to indicate that the problem has been caused
// by incorrect flags by the user, and as such, the help screen should be printed.
type UsageError struct {
	error
}

func (ue *UsageError) Unwrap() error {
	return ue.error
}

func NewUsageError(err error) *UsageError {
	return &UsageError{error: err}
}

func NewUsageErrorf(f string, args ...interface{}) *UsageError {
	return &UsageError{error: fmt.Errorf(f, args...)}
}

// IdentityError is an error that indicates that there was some problem
// loading or using the identity and credentials of the user.
//
// It doesn't really belong to kflags, except it's a common error returned
// by cli commands, and requires some special handling.
type IdentityError struct {
	error
}

func (ie *IdentityError) Unwrap() error {
	return ie.error
}

func NewIdentityError(err error) *IdentityError {
	return &IdentityError{error: err}
}

// An ErrorHandler takes an error as input, transforms it, and returns an error as output.
//
// This can be used, for example, to improve the readability of an error, ignore it, or turn
// a low level error into a higher level error (eg, "cannot open file" -> "credentials missing").
//
// For each error, all error handlers configured are executed in the order they were originally
// supplied, each taking as input the output of the previous handler.
//
// For an example, look for HandleIdentityError.
type ErrorHandler func(err error) error

// Printer is a function capable of printing Printf like strings.
type Printer func(format string, v ...interface{})

// An initialization function capable of preparing internal objects once flags have been parsed.
type Init func() error

// Runner is a function capable of parsing argv according to the supplied FlagSet,
// log problems using the supplied Printer, initialize internal object with the
// parsed flags by invoking Init, and finally running the program.
type Runner func(FlagSet, Printer, Init)
