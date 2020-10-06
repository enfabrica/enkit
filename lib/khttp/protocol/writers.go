package protocol

import (
	"bytes"
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

// Returned by Read operations to provide details on the failed operation,
// by wrapping an existing error.
type HTTPError struct {
	error

	URL  string
	Resp *http.Response
}

func (e *HTTPError) Unwrap() error {
	return e.error
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP request for %s: %v", e.URL, e.error)
}

type ResponseHandler func(url string, resp *http.Response, err error) error

type StatusChecker func(code int, message string) error

type StatusCheckers []StatusChecker

func (sc StatusCheckers) Run(url string, resp *http.Response) error {
	// Goal of the unusual loop here is to succeed if at least one of the checkers succeeds.
	var errs []error
	for _, check := range sc {
		err := check(resp.StatusCode, resp.Status)
		if err == nil {
			break
		}
		errs = append(errs, err)
	}
	if len(errs) >= len(sc) {
		return &HTTPError{
			error: multierror.New(errs),
			Resp:  resp,
			URL:   url,
		}
	}
	return nil
}

func StatusOK(code int, message string) error {
	if code != http.StatusOK {
		return fmt.Errorf("status is not OK - %s", message)
	}
	return nil
}

func StatusRange(min, max int) StatusChecker {
	return func(code int, message string) error {
		if code < min || code > max {
			return fmt.Errorf("status %d outside valid range - < %d, and > %d - %s", code, min, max, message)
		}
		return nil
	}
}

func StatusValue(expected int) StatusChecker {
	return func(code int, message string) error {
		if code != expected {
			return fmt.Errorf("status %d is unexpected - != %d - %s", code, expected, message)
		}
		return nil
	}
}

type ResponseReader func(r io.Reader) error

func Reader(r ResponseReader, checker ...StatusChecker) ResponseHandler {
	if len(checker) == 0 {
		checker = []StatusChecker{StatusOK}
	}

	return func(url string, resp *http.Response, err error) error {
		if err != nil {
			return err
		}

		if err := StatusCheckers(checker).Run(url, resp); err != nil {
			return err
		}

		return r(resp.Body)
	}
}

func Read(opener ResponseOpener, checker ...StatusChecker) ResponseHandler {
	if len(checker) == 0 {
		checker = []StatusChecker{StatusOK}
	}

	return func(url string, resp *http.Response, err error) error {
		if err != nil {
			return err
		}

		if err := StatusCheckers(checker).Run(url, resp); err != nil {
			return err
		}

		writer, err := opener(resp)
		if err != nil {
			return fmt.Errorf("couldn't open output descriptor to download %s - %w", url, err)
		}

		if _, err := io.Copy(writer, resp.Body); err != nil {
			return fmt.Errorf("while downloading url %s - error %w", url, err)
		}

		return writer.Close()
	}
}

// Opener is a function able to open an output stream.
//
// The size argument indicates how many bytes are expected in this stream.
// The parameter cannot be trusted, as the remote server is allowed to return any value.
// 0 indicates the amount of bytes is unknown.
//
// Opener returns an io.WriteCloser(), usable to write the data out, or an error.
type ResponseOpener func(resp *http.Response) (io.WriteCloser, error)

// File returns an Opener able to write to a file.
// path if the path of the file.
//
// Note that the file will not be opened until the Opener is actually invoked,
// at which point an error may be returned.
func File(path string) ResponseOpener {
	return func(resp *http.Response) (io.WriteCloser, error) {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}

type nullWriter struct{}

func (nf nullWriter) Write(data []byte) (int, error) {
	return len(data), nil
}
func (nf nullWriter) Close() error {
	return nil
}

var null = &nullWriter{}

// Null is an Opener that will discard all the data that it is passed.
func Null() ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return null, nil
	}
}

type stringWriter struct {
	strings.Builder
	dest *string
}

func (sw *stringWriter) Close() error {
	*sw.dest = sw.String()
	return nil
}

// String returns an opener able to store data in a string.
func String(dest *string) ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return &stringWriter{dest: dest}, nil
	}
}

type bufferWriter struct {
	bytes.Buffer
	dest *[]byte
}

func (bw *bufferWriter) Close() error {
	*bw.dest = bw.Buffer.Bytes()
	return nil
}

// Buffer returns an opener able to write all the data in a []byte array.
func Buffer(dest *[]byte) ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return &bufferWriter{dest: dest}, nil
	}
}

type limit struct {
	io.WriteCloser
	limit, written int64
}

func (l *limit) Write(data []byte) (int, error) {
	value := atomic.AddInt64(&l.written, int64(len(data)))
	if value > l.limit {
		return 0, fmt.Errorf("exceeded write limits - maximum set to %d, would write %d total", l.limit, value)
	}
	return l.WriteCloser.Write(data)
}

// Limit returns an opener that will limit the total amount of bytes written to the max specified.
//
// For example, by creating an opener like:
//
//   Get(..., Limit(4096, String(&data)), ...)
//
// Get will stop the download and return an error if more than 4096 bytes of data are returned
// by the remote server.
func Limit(max int64, nested ResponseOpener) ResponseOpener {
	return func(resp *http.Response) (io.WriteCloser, error) {
		if resp.ContentLength > 0 && resp.ContentLength > max {
			return nil, fmt.Errorf("request would return %d bytes - more than the limit of %d", resp.ContentLength, max)
		}
		iow, err := nested(resp)
		if err != nil {
			return nil, err
		}
		return &limit{WriteCloser: iow, limit: max, written: 0}, nil
	}
}

// WriteCloser allows to pass a simple io.WriteCloser without creating an Opener.
func WriteCloser(wc io.WriteCloser) ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return wc, nil
	}
}

type ignoreClose struct {
	io.Writer
}

func (ic ignoreClose) Close() error {
	return nil
}

// Writer allows to pass a simple io.Writer without creating an Opener.
func Writer(wc io.Writer) ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return &ignoreClose{wc}, nil
	}
}

type CallbackWriter struct {
	bytes.Buffer
	cb func([]byte) error
}

func (bw *CallbackWriter) Close() error {
	return bw.cb(bw.Buffer.Bytes())
}

func NewCallbackWriter(cb func([]byte) error) *CallbackWriter {
	return &CallbackWriter{cb: cb}
}

// Callback will invoke the specified callback once the write has completed.
func Callback(cb func([]byte) error) ResponseOpener {
	return func(*http.Response) (io.WriteCloser, error) {
		return NewCallbackWriter(cb), nil
	}
}

type CloseCallbackWriter struct {
	resp *http.Response
	cb   func(*http.Response) error
}

func (cb CloseCallbackWriter) Write(data []byte) (int, error) {
	return len(data), nil
}
func (cb CloseCallbackWriter) Close() error {
	return cb.cb(cb.resp)
}

// OnClose will invoke the specified callback on close only.
func OnClose(cb func(*http.Response) error) ResponseOpener {
	return func(resp *http.Response) (io.WriteCloser, error) {
		return CloseCallbackWriter{resp: resp, cb: cb}, nil
	}
}

type ChainWriter []io.WriteCloser

func (cw ChainWriter) Write(data []byte) (int, error) {
	for _, w := range cw {
		written, err := w.Write(data)
		if err != nil {
			return written, err
		}
	}
	return len(data), nil
}
func (cw ChainWriter) Close() error {
	var errs []error
	for _, w := range cw {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return multierror.New(errs)
}

func Chain(openers ...ResponseOpener) ResponseOpener {
	return func(resp *http.Response) (io.WriteCloser, error) {
		errs := []error{}
		writers := []io.WriteCloser{}
		for _, o := range openers {
			w, err := o(resp)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			writers = append(writers, w)
		}

		return ChainWriter(writers), multierror.New(errs)
	}
}
