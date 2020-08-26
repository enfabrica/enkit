package progress

import (
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"github.com/cheggaaa/pb/v3/termutil"
	"io"
)

func TerminalWidth() (int, error) {
	return termutil.TerminalWidth()
}

type Handler interface {
	Step(fmt string, args ...interface{})
	Reader(reader io.ReadCloser, total int64) io.ReadCloser
	Writer(writer io.WriteCloser, total int64) io.WriteCloser
	Done()
}

type Factory func() Handler

type Discard struct{}

func (dp *Discard) Done()                                {}
func (dp *Discard) Step(fmt string, args ...interface{}) {}
func (dp *Discard) Reader(reader io.ReadCloser, total int64) io.ReadCloser {
	return reader
}
func (dp *Discard) Writer(writer io.WriteCloser, total int64) io.WriteCloser {
	return writer
}
func NewDiscard() Handler {
	return &Discard{}
}

type Bar pb.ProgressBar

func (bp *Bar) Step(fstring string, args ...interface{}) {
	(*pb.ProgressBar)(bp).Set("prefix", fmt.Sprintf(fstring+" ", args...))
	(*pb.ProgressBar)(bp).Write()
}
func (bp *Bar) Reader(reader io.ReadCloser, total int64) io.ReadCloser {
	bar := (*pb.ProgressBar)(bp)
	bar.SetTotal(total)
	return bar.NewProxyReader(reader)
}
func (bp *Bar) Writer(writer io.WriteCloser, total int64) io.WriteCloser {
	bar := (*pb.ProgressBar)(bp)
	bar.SetTotal(total)
	return bar.NewProxyWriter(writer)
}
func (bp *Bar) Done() {
	(*pb.ProgressBar)(bp).Finish()
}

func WriterCreator(h Handler, w io.WriteCloser) func(int64) io.WriteCloser {
	return func(size int64) io.WriteCloser {
		return h.Writer(w, size)
	}
}

func NewBar() Handler {
	return (*Bar)(pb.Full.Start64(0))
}
