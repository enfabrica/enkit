package buildevent

import (
	"github.com/enfabrica/enkit/lib/multierror"
)

// errslice accumulates errors into a slice in a threadsafe manner.
type errslice struct {
	errs    []error
	errChan chan error
	done    chan struct{}
}

// newErrslice returns a started errslice.
func newErrslice() *errslice {
	s := &errslice{
		errs:    []error{},
		errChan: make(chan error),
		done:    make(chan struct{}),
	}
	go s.collect()
	return s
}

// Append records an error. Append() can't be called after Close().
func (s *errslice) Append(err error) {
	s.errChan <- err
}

func (s *errslice) collect() {
	defer close(s.done)
	for err := range s.errChan {
		if err != nil {
			s.errs = append(s.errs, err)
		}
	}
}

// Close stops new errors from being added, and returns an error if this
// errslice contains any errors.
func (s *errslice) Close() error {
	close(s.errChan)
	<-s.done
	return multierror.New(s.errs)
}
