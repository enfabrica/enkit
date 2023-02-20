[![Go Reference](https://pkg.go.dev/badge/github.com/enfabrica/enkit/lib/retry.svg)](https://pkg.go.dev/github.com/enfabrica/enkit/lib/retry)

# Overview

The `retry` library is a simple golang library to implement retry logic
in a simple, configurable, and reliable way. 

For example, let's say you have a `Scrape()` function, to scrape content
from a remote website: `func Scrape() error`. Scraping fails at times, and
you want to re-try this function up to 10 times, waiting 1 second in between
attempts. You can write:

    import (
	"github.com/enfabrica/enkit/lib/retry"
	"fmt"
	"time"
    )

    func DoWork() {
    	...
      	if err := retry.New(retry.WithAttempts(10), retry.WithDelay(1 * time.Second)).Run(Scrape); err != nil {
		return fmt.Errorf("scraping failed after 10 attempts: %w", err)
      	}
    }

The main features of the retry library are:

  1. Fuzzies delays by default (but configurable) - this is important to avoid
     the [thundering herd problem](https://en.wikipedia.org/wiki/Thundering_herd_problem) in large systems.
  2. Allows the configuration of attempts, delay, logger, fuzzying, random
     number generator, a log message, and time source to simplify testing.
  3. Captures the last n errors (configurable) in a [multierror](https://github.com/enfabrica/enkit/tree/master/lib/multierror),
     for user friendly messages as well as easy processing of the errors.
  4. Allows the function to stop retries, with `return Fatal(err)`.
  5. Allows to implement retry logic in functions that cannot block or sleep,
     by invoking `Once` (instead of Run) and re-scheduling the call later.
  6. Allows to access the original errors returned by the function using normal
     `errors.Unwrap`, or `errors.As` or `errors.Is`, and wraps errors so it
     is possible to distinguish between a fatal error returned by the function
     (`FatalError`) or having exhausted the attempts (`ExaustedError`)
  7. Allows to parse the retry parameters from the command line. See example below.

Command line example:

    import (
        ...
	"github.com/enfabrica/enkit/lib/retry"
        ...
	"github.com/enfabrica/enkit/lib/kflags"
        ...
        "flag"
    )

    func main() {
        retryFlags := retry.DefaultFlags()

    	// "scrape-" is a prefix to give to the added flags.
    	//
    	// If using cobra, you can use &kcobra.FlagSet{FlagSet: ...} instead, from
    	// github.com/enfabrica/enkit/lib/kflags/kcobra.
        retryFlags.Register(&kflags.GoFlagSet{FlagSet: flag.CommandLine}, "scrape-")
        ...
        flag.Parse()
        ...

        if err := retry.New(retry.FromFlags(retryFlags)).Run(func () error {
		return Scrape()
	}); err != nil {
		log.Fatal("scrape failed: %v", err)
	}
    }

In the example above, running the command with `--help` would show a few extra flags like
`--scrape-retry-at-most`, `--scrape-retry-max-errors`, `--scrape...`, as per retry.Register
function definition.

# Documentation

All the documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/enfabrica/enkit/lib/retry).
