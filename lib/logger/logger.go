package logger

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// Logger is the interface used by the enkit libraries to log messages.
//
// The logrus library can be used out of the box without additional
// glue. The golang log library is used by default, through DefaultLogger.
// DefaultLogger can be used to pass an arbitrary log function as well.
type Logger interface {
	// Use for messages that are pretty much only useful when debugging
	// specific behaviors. Should be used rarely.
	// Expect debug messages to be discarded.
	Debugf(format string, args ...interface{})
	// Use for messages that clearly a) outline a problem that b) the operator MUST act upon.
	// Errors are generally reserved for things that cause the program to fail and must be fixed.
	Errorf(format string, args ...interface{})
	// Use for messages that a) outline a problem that b) is actionable, and c) the operator
	// may want to act upon - even though the program may have recovered or continued without
	// intervention (in degraded mode, or without some functionality, ...).
	Warnf(format string, args ...interface{})
	// Use for messages that are either not actionable, or require no action by the operator.
	Infof(format string, args ...interface{})

	SetOutput(writer io.Writer)
}

// Forwardable is the interface implemented by any logger capable
// of forwarding, upon request, buffered messages to another logger.
type Forwardable interface {
	Forward(Logger)
}

// Printer is a Printf like function.
type Printer func(format string, args ...interface{})

// LogLines breaks a buffer into lines and logs each one of them with the
// specified indentation and printer.
func LogLines(logger Printer, buffer, indent string) {
	scanner := bufio.NewScanner(strings.NewReader(buffer))
	for scanner.Scan() {
		logger("%s", indent+scanner.Text())
	}
}

// DefaultLogger implements the Logger interface.
//
// Printer must be provided. Use log.Printf to rely on default golang logging, with:
//    logger := &DefaultLoger{Printer: log.Printf}
//
type DefaultLogger struct {
	Printer Printer
	Setter  func(writer io.Writer)
}

func (dl DefaultLogger) Printf(format string, args ...interface{}) {
	dl.Printer(format, args...)
}
func (dl DefaultLogger) Debugf(format string, args ...interface{}) {
	dl.Printf("[debug] "+format, args...)
}
func (dl DefaultLogger) Infof(format string, args ...interface{}) {
	dl.Printf("[info] "+format, args...)
}
func (dl DefaultLogger) Errorf(format string, args ...interface{}) {
	dl.Printf("[error] "+format, args...)
}
func (dl DefaultLogger) Warnf(format string, args ...interface{}) {
	dl.Printf("[warning] "+format, args...)
}

func (dl DefaultLogger) SetOutput(output io.Writer) {
	if dl.Setter != nil {
		dl.Setter(output)
	}
}

// Nil is a pre-defined logger that will discard all the output.
//
// Replacing the Nil global with something else can allow seeing
// log messages that would normally be discarded.
var Nil Logger = &NilLogger{}

// NilLogger is a logger that discards all messages.
//
// Prefer using logger.Nil to instantiating your copy of &NilLogger{}.
type NilLogger struct{}

func (dl NilLogger) Printf(format string, args ...interface{}) {
}
func (dl NilLogger) Debugf(format string, args ...interface{}) {
}
func (dl NilLogger) Infof(format string, args ...interface{}) {
}
func (dl NilLogger) Errorf(format string, args ...interface{}) {
}
func (dl NilLogger) Warnf(format string, args ...interface{}) {
}
func (dl NilLogger) SetOutput(output io.Writer) {
}

// IndentedError reformats the error to have indented new lines.
type IndentedError struct {
	indent string
	err    error
}

func (ie *IndentedError) Unwrap() error {
	return ie.err
}

func (ie *IndentedError) Error() string {
	return IndentLines(ie.err.Error(), ie.indent)
}

// NewIndentedError wraps an error into an object that ensures that when
// the error is converted to a string and printed, via the Error method, every
// line of the error message is indented with indent.
//
// For example, let's say you have an error err that converted with
// fmt.Printf(... %s ...) leads to the following output:
//
//   An error occured in executing function, stack trace follows.
//     Function1() ...
//     Function2() ...
//
// Using NewIndentedError(err, "| "), would lead to:
//
//   | An error occured in executing function, stack trace follows.
//   |   Function1() ...
//   |   Function2() ...
//
// Further, the error returned by NewIntendedError implements the
// Unwrap interface, allowing programmatic access to the original error.
func NewIndentedError(err error, indent string) *IndentedError {
	return &IndentedError{
		err:    err,
		indent: indent,
	}
}

// IndentLines indents each line in the buffer with the specified string.
func IndentLines(buffer, indent string) string {
	return indent + strings.ReplaceAll(strings.TrimSuffix(buffer, "\n"), "\n", "\n"+indent)
}

// Just like IndentLines, but converts all non-space / non-ascii printable characters to \xHH sequences.
func IndentAndQuoteLines(buffer, indent string) string {
	conv := [...]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}
	bindent := []byte(indent)
	cr := false
	result := append(make([]byte, 0, len(buffer)*2), bindent...)
	for _, ch := range []byte(buffer) {
		// Normalize \r\n to just \n.
		if cr {
			cr = false
			if ch == '\n' {
				continue
			}
		}
		// Escape all non ascii characters.
		switch {
		case (ch >= 0x20 && ch <= 0x7E) || ch == '\t':
			result = append(result, ch)
		case ch == '\n':
			result = append(result, ch)
			result = append(result, bindent...)
		case ch == '\r':
			result = append(result, '\n')
			result = append(result, bindent...)
			cr = true
		default:
			result = append(result, []byte("\\x")...)
			result = append(result, conv[(ch>>4)&0xf])
			result = append(result, conv[ch&0xf])
		}
	}
	return string(result)
}

func SetCtx(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, "logger", l)
} 

func GetCtx(ctx context.Context) Logger {
	l, ok := ctx.Value("logger").(Logger)
	if !ok {
		return Nil
	}
	return l
}