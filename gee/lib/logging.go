package lib

import (
	"github.com/xo/terminfo"
	"os"
	"strings"
)

type Logger struct {
	ti        *terminfo.Terminfo
	verbosity int
}

func NewLogger() *Logger {
	var err error
	logger := new(Logger)
	logger.ti, err = terminfo.LoadFromEnv()
	if err != nil {
		panic(err)
	}
	return logger
}

type ColorAttr struct {
	fg      int
	bg      int
	bold    bool
	reverse bool
}

func (logger *Logger) PrintColorAttr(c ColorAttr) {
	os.Stderr.WriteString(logger.ti.Colorf(c.fg, c.bg, ""))
	if c.bold {
    logger.ti.Fprintf(os.Stderr, terminfo.EnterBoldMode)
	}
	if c.reverse {
    logger.ti.Fprintf(os.Stderr, terminfo.EnterReverseMode)
	}
}

func (logger *Logger) Print(c ColorAttr, lines []string) {
	logger.PrintColorAttr(c)
  columns := logger.ti.Num(terminfo.Columns)
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

func (logger *Logger) Debug(lines []string) {
	logger.Print(ColorAttr{fg: 2}, lines)
}

func (logger *Logger) Info(lines []string) {
	logger.Print(ColorAttr{fg: 1}, lines)
}

func (logger *Logger) Warning(lines []string) {
	logger.Print(ColorAttr{fg: 11}, lines)
}

func (logger *Logger) Error(lines []string) {
	logger.Print(ColorAttr{fg: 15, bg: 3, bold: true}, lines)
}

func (logger *Logger) Fatal(lines []string) {
	logger.Error(lines)
	os.Exit(1)
}

func (logger *Logger) Banner(lines []string) {
  columns := logger.ti.Num(terminfo.Columns)
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
