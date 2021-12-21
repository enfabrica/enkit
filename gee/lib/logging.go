package lib

import (
  "fmt"
	"github.com/xo/terminfo"
	"os"
	"strings"
  "syscall"
  "unsafe"
)

// for TIOCGWINSZ ioctl:
type winsize struct {
  row uint16
  col uint16
  xpixel uint16
  ypixel uint16
}

type Logger struct {
	ti        *terminfo.Terminfo
	verbosity int
  columns   int
}

var logger *Logger = nil

func NewLogger() *Logger {
	var err error
	logger := new(Logger)
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

func GetLogger() *Logger {
  if logger == nil {
    logger = NewLogger()
  }
  return logger
}

func (logger *Logger) DumpTermInfo() {
  logger.ti.Fprintf(os.Stderr, terminfo.EnterBoldMode)
  logger.ti.Fprintf(os.Stderr, terminfo.EnterBoldMode)
  fmt.Printf("from: %s\n", logger.ti.File)
  logger.PrintColorAttr(ColorAttr{fg: 2, bold: true})
  fmt.Printf("colors: %d\n", logger.ti.Num(terminfo.MaxColors))
  fmt.Printf("columns: %d\n", logger.columns)
}

type ColorAttr struct {
	fg      int
	bg      int
	bold    bool
	reverse bool
}

func (logger *Logger) PrintColorAttr(c ColorAttr) {
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

func (logger *Logger) Print(c ColorAttr, lines []string) {
	logger.PrintColorAttr(c)
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

func (logger *Logger) Debug(lines ...string) {
	logger.Print(ColorAttr{fg: 2}, lines)
}

func (logger *Logger) Info(lines ...string) {
	logger.Print(ColorAttr{fg: 1}, lines)
}

func (logger *Logger) Warning(lines ...string) {
	logger.Print(ColorAttr{fg: 11}, lines)
}

func (logger *Logger) Error(lines ...string) {
	logger.Print(ColorAttr{fg: 15, bg: 3, bold: true}, lines)
}

func (logger *Logger) Fatal(lines ...string) {
	logger.Error(lines...)
	os.Exit(1)
}

func (logger *Logger) Banner(lines ...string) {
  columns := logger.columns
	logger.PrintColorAttr(ColorAttr{bold: true, fg: 15, bg: 3})
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	for _, line := range lines {
		os.Stderr.WriteString("# ")
		os.Stderr.WriteString(line)
		os.Stderr.WriteString("\n")
	}
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
  logger.ti.Fprintf(os.Stderr, terminfo.ExitAttributeMode)
}
