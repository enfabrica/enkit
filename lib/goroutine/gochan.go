package goroutine

// ErrorChannel is a simple "chan error" with a couple convenience methods,
// and strong typing to help the compiler.
type ErrorChannel chan error

func (ec ErrorChannel) Channel() chan error {
	return (chan error)(ec)
}

func (ec ErrorChannel) Terminated() bool {
	return len(ec) > 0
}

// Run will start a go() coroutine for the specified function, and return an error channel.
func Run(goroutine func() error) ErrorChannel {
	ch := make(chan error, 1)
	go func() {
		ch <- goroutine()
	}()

	return ch
}

// WaitAll runs a goroutine for each function, waits for each to complete, and returns all errors.
func WaitAll(goroutine ...func() error) []error {
	type eix struct {
		err error
		ix  int
	}

	ec := make(chan eix, len(goroutine))
	for ix, g := range goroutine {
		index := ix
		routine := g
		go func() {
			ec <- eix{
				err: routine(),
				ix:  index,
			}
		}()
	}

	var errs []error
	for i := 0; i < len(goroutine); i++ {
		result := <-ec
		if result.err != nil {
			if errs == nil {
				errs = make([]error, len(goroutine))
			}
			errs[result.ix] = result.err
		}
	}
	return errs
}

// WaitFirst runs a goroutine for each function, returns as soon as all have completed, or one errors out.
func WaitFirstError(goroutine ...func() error) error {
	ec := make(chan error, len(goroutine))
	for _, g := range goroutine {
		routine := g
		go func() {
			ec <- routine()
		}()
	}

	for i := 0; i < len(goroutine); i++ {
		if err := <-ec; err != nil {
			return err
		}
	}
	return nil
}
