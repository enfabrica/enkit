package lib

import (
  "fmt"
	"github.com/xo/terminfo"
  "github.com/alessio/shellescape"
	"os"
	"strings"
  "syscall"
  "unsafe"
)

type singletonLogger struct {
	ti        *terminfo.Terminfo
	verbosity int
  columns   int
}

var logger *singletonLogger = nil

// for TIOCGWINSZ ioctl:
type winsize struct {
  row uint16
  col uint16
  xpixel uint16
  ypixel uint16
}

func newLogger() *singletonLogger {
	var err error
	logger := new(singletonLogger)
	logger.ti, err = terminfo.LoadFromEnv()
	if err != nil {
		panic(err)
	}
  ws := &winsize{}
  retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
    uintptr(syscall.Stderr),
    uintptr(syscall.TIOCGWINSZ),
    uintptr(unsafe.Pointer(ws)))
  if int(retCode) == -1 {
    panic(errno)
  }
  logger.columns = int(ws.col)

	return logger
}

// Get a handle to the logger singleton.
func Logger() *singletonLogger {
  if logger == nil {
    logger = NewLogger()
  }
  return logger
}

// Returns the width of the terminal in columns.
func (logger *singletonLogger) GetColumns int {
  return logger.columns
}

// Returns the maximum number of colors as specified by TERMINFO.
func (logger *singletonLogger) GetMaxColors int {
  return logger.ti.Num(terminfo.MaxColors)
}

// A structure for specifying text colors and attributes.
type ColorAttr struct {
	fg      int
	bg      int
	bold    bool
	reverse bool
}

func (logger *singletonLogger) printColorAttr(c ColorAttr) {
  if c.fg != 0 {
    logger.ti.Fprintf(os.Stderr, terminfo.SetAForeground, c.fg)
  }
  if c.bg != 0 {
    logger.ti.Fprintf(os.Stderr, terminfo.SetABackground, c.bg)
  }
	if c.bold {
    logger.ti.Fprintf(os.Stderr, terminfo.EnterBoldMode)
	}
	if c.reverse {
    logger.ti.Fprintf(os.Stderr, terminfo.EnterReverseMode)
	}
}

// Generic logging function with arbitrary color/attributes.
func (logger *singletonLogger) Log(c ColorAttr, lines []string) {
	logger.printColorAttr(c)
  columns := logger.columns
	for _, line := range lines {
		os.Stderr.WriteString(line)
		// for terminals that don't support clear-to-end:
		padding := columns - len(line) - 1
		if padding > 0 {
			os.Stderr.WriteString(strings.Repeat(" ", padding))
		}
		os.Stderr.WriteString("\n")
	}
  logger.ti.Fprintf(os.Stderr, terminfo.ExitAttributeMode)
}

// Log debug messages.
func (logger *singletonLogger) Debug(lines ...string) {
	logger.Log(ColorAttr{fg: 2}, lines)
}

// Log a properly shell-escaped command line.
func (logger *singletonLogger) Command(args ...string) {
  logger.Log(ColorAttr{bold: true, fg: 12, reverse: true}, []string{"CMD: " + shellescape.QuoteCommand(args)})
}

// Log info messages.
func (logger *singletonLogger) Info(lines ...string) {
	logger.Log(ColorAttr{fg: 1}, lines)
}

// Log warning messages.
func (logger *singletonLogger) Warning(lines ...string) {
	logger.Log(ColorAttr{fg: 11}, lines)
}

// Log error messages.
func (logger *singletonLogger) Error(lines ...string) {
	logger.Log(ColorAttr{fg: 15, bg: 3, bold: true}, lines)
}

// Log fatal error messages, then terminate.
func (logger *singletonLogger) Fatal(lines ...string) {
	logger.Error(lines...)
	os.Exit(1)
}

// Create a banner log message.
func (logger *singletonLogger) Banner(lines ...string) {
  columns := logger.columns
	logger.printColorAttr(ColorAttr{bold: true, fg: 15, bg: 3})
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	for _, line := range lines {
		os.Stderr.WriteString("# ")
		os.Stderr.WriteString(line)
		os.Stderr.WriteString("\n")
	}
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
  logger.ti.Fprintf(os.Stderr, terminfo.ExitAttributeMode)
}
