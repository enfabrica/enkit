package lib

import (
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/spf13/viper"
	"github.com/xo/terminfo"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type singletonLogger struct {
	ti        *terminfo.Terminfo
	verbosity int
	columns   int
	logFile   *log.Logger
}

var logger *singletonLogger = nil

// for TIOCGWINSZ ioctl:
type winsize struct {
	row    uint16
	col    uint16
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
		logger = newLogger()
	}
	return logger
}

func OpenFile(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	logger.logFile = log.New(file, viper.GetString("upstream")+":"+viper.GetString("repository")+": ", log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}

// Returns the width of the terminal in columns.
func (logger *singletonLogger) GetColumns() int {
	return logger.columns
}

// Returns the maximum number of colors as specified by TERMINFO.
func (logger *singletonLogger) GetMaxColors() int {
	return logger.ti.Num(terminfo.MaxColors)
}

// A structure for specifying text colors and attributes.
type ColorAttr struct {
	fg      int
	bg      int
	bold    bool
	reverse bool
}

var (
	colorDebug   = ColorAttr{fg: 2}
	colorCommand = ColorAttr{bold: true, fg: 12, reverse: true}
	colorInfo    = ColorAttr{fg: 1}
	colorWarn    = ColorAttr{fg: 11}
	colorError   = ColorAttr{fg: 15, bg: 3, bold: true}
	colorBanner  = ColorAttr{fg: 15, bg: 3, bold: true}
)

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

func (logger *singletonLogger) LogToFile(logtype string, lines []string) {
	if logger.logFile != nil {
		for _, line := range lines {
			logger.logFile.Output(2, logtype+": "+line)
		}
	}
}

// Log a diagnostic message, to the log file if available.
func (logger *singletonLogger) Debug(lines ...string) {
	logger.LogToFile("DEBUG", lines)
	if logger.logFile == nil {
		// write to terminal if logFile isn't available.
		logger.Log(colorDebug, lines)
	}
}

// Log a properly shell-escaped command line.
func (logger *singletonLogger) Command(args ...string) {
	message := shellescape.QuoteCommand(args)
	logger.LogToFile("CMD", []string{message})
	logger.Log(colorCommand, []string{"CMD: " + message})
}

// Log info messages.
func (logger *singletonLogger) Info(lines ...string) {
	logger.LogToFile("INFO", lines)
	logger.Log(colorInfo, lines)
}

// Log warning messages.
func (logger *singletonLogger) Warn(lines ...string) {
	logger.LogToFile("WARN", lines)
	logger.Log(colorWarn, lines)
}

// Log error messages.
func (logger *singletonLogger) Error(lines ...string) {
	logger.LogToFile("ERROR", lines)
	logger.Log(colorError, lines)
}

// Log fatal error messages, then terminate.
func (logger *singletonLogger) Fatal(lines ...string) {
	logger.LogToFile("FATAL", lines)
	logger.Log(colorError, lines)
	os.Exit(1)
}

// Create a banner log message.
func (logger *singletonLogger) Banner(lines ...string) {
	logger.LogToFile("BANNER", lines)
	columns := logger.columns
	logger.printColorAttr(colorBanner)
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	for _, line := range lines {
		os.Stderr.WriteString("# ")
		os.Stderr.WriteString(line)
		os.Stderr.WriteString("\n")
	}
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	logger.ti.Fprintf(os.Stderr, terminfo.ExitAttributeMode)
}

// Log a formatted diagnostic message, to the log file if available.
func (logger *singletonLogger) Debugf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("DEBUG", []string{message})
	if logger.logFile == nil {
		// write to terminal if logFile isn't available.
		logger.Log(colorDebug, []string{message})
	}
}

// Log a formatted info message.
func (logger *singletonLogger) Infof(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("INFO", []string{message})
	logger.Log(colorInfo, []string{message})
}

// Log a formatted warning message.
func (logger *singletonLogger) Warnf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("WARN", []string{message})
	logger.Log(colorWarn, []string{message})
}

// Log a formatted error message.
func (logger *singletonLogger) Errorf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("ERROR", []string{message})
	logger.Log(colorError, []string{message})
}

// Log a formatted fatal error message, and terminate.
func (logger *singletonLogger) Fatalf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("FATAL", []string{message})
	logger.Log(colorError, []string{message})
	os.Exit(1)
}

// Create a banner log message.
func (logger *singletonLogger) Bannerf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	logger.LogToFile("BANNER", []string{message})
	columns := logger.columns
	logger.printColorAttr(colorBanner)
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	os.Stderr.WriteString("# ")
	os.Stderr.WriteString(message)
	os.Stderr.WriteString("\n")
	os.Stderr.WriteString(strings.Repeat("#", columns-1) + "\n")
	logger.ti.Fprintf(os.Stderr, terminfo.ExitAttributeMode)
}
