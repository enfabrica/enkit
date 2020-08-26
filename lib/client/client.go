package client

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/progress"
	"github.com/spf13/pflag"
	"log"
)

type CommonFlags struct {
	Quiet      bool
	NoProgress bool
}

func (cf *CommonFlags) Register(store *pflag.FlagSet) {
	store.BoolVarP(&cf.Quiet, "quiet", "q", false, "Disable logging, be quiet")
	store.BoolVarP(&cf.NoProgress, "no-progress", "P", false, "Disable the progress bar")
}

// Options() creates a new CommonOptions object.
//
// logger, if provided, is the logger to use to send messages to the user if
// the command is not running with the --quiet option.
func (cf *CommonFlags) Options(clog logger.Logger) *CommonOptions {
	options := DefaultCommonOptions()

	if cf.NoProgress {
		options.Progress = progress.NewDiscard
	} else {
		options.Progress = progress.NewBar
	}

	if cf.Quiet {
		options.Logger = logger.Nil
	} else {
		if clog == nil {
			clog = &logger.DefaultLogger{Printer: log.Printf}
		}
		options.Logger = clog
	}

	return options
}

type CommonOptions struct {
	Progress progress.Factory
	Logger   logger.Logger

	TerminalWidth int
	MaxPathLength int
}

func (o *CommonOptions) ShortPath(path string) string {
	if o.MaxPathLength == 0 || len(path) < o.MaxPathLength {
		return path
	}
	if o.MaxPathLength < 3 {
		return path[len(path)-o.MaxPathLength:]
	}
	return "..." + path[len(path)-(o.MaxPathLength-3):]
}

func DefaultCommonOptions() *CommonOptions {
	options := &CommonOptions{}

	width, err := progress.TerminalWidth()
	if err != nil {
		options.MaxPathLength = 20
	} else {
		options.MaxPathLength = width / 5
	}
	options.TerminalWidth = width
	return options
}
