package ccontext

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/progress"
)

type Context struct {
	Progress progress.Factory
	Logger   logger.Logger

	TerminalWidth int
	MaxPathLength int
}

func (o *Context) ShortPath(path string) string {
	if o.MaxPathLength == 0 || len(path) < o.MaxPathLength {
		return path
	}
	if o.MaxPathLength < 3 {
		return path[len(path)-o.MaxPathLength:]
	}
	return "..." + path[len(path)-(o.MaxPathLength-3):]
}

func DefaultContext() *Context {
	options := &Context{}

	width, err := progress.TerminalWidth()
	if err != nil {
		options.MaxPathLength = 20
	} else {
		options.MaxPathLength = width / 5
	}
	options.TerminalWidth = width
	return options
}
