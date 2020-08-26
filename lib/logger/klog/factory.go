// +build !windows

package klog

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/tchap/zapext/zapsyslog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log/syslog"
	"os"
	"strings"
)

type Logger struct {
	*zap.SugaredLogger
}

func (l *Logger) SetOutput(writer io.Writer) {
}

func Syslog(tag string, sl syslog.Priority, zl zapcore.Level) (zapcore.Core, error) {
	writer, err := syslog.New(sl|syslog.LOG_USER, tag)
	if err != nil {
		return nil, fmt.Errorf("could not initialize syslog - %w", err)
	}
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	return zapsyslog.NewCore(zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return lvl == zl }), encoder, writer), nil
}

type Flags struct {
	ConsoleLevel string
	SyslogLevel  string
	Verbosity    int
}

func DefaultFlags() *Flags {
	return &Flags{
		ConsoleLevel: "warn",
		SyslogLevel:  "info",
		Verbosity:    0,
	}
}

func (cf *Flags) Register(flags kflags.FlagSet, prefix string) *Flags {
	flags.StringVar(&cf.ConsoleLevel, prefix+"loglevel-console", cf.ConsoleLevel, "Can be debug, info, warn, error. Indicates the minimum severity of messages to log on the console")
	flags.StringVar(&cf.SyslogLevel, prefix+"loglevel-syslog", cf.SyslogLevel, "Can be debug, info, warn, error. Indicates the minimum severity of messages to log in syslog")
	flags.IntVar(&cf.Verbosity, prefix+"verbosity", cf.Verbosity, "Increases the verbosity level of logs by the specified amount")
	return cf
}

type options struct {
	minConsole zapcore.Level
	minSyslog  zapcore.Level
}

type Modifier func(o *options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(o *options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

type Level struct {
	Name  string
	Value zapcore.Level
}

type Levels []Level

func (levels Levels) Find(name string) (int, *Level) {
	name = strings.TrimSpace(strings.ToLower(name))
	for ix, level := range levels {
		if strings.HasPrefix(level.Name, name) {
			return ix, &levels[ix]
		}
	}
	return 0, nil
}

func (levels Levels) String() string {
	keys := []string{}
	for _, key := range levels {
		keys = append(keys, key.Name)
	}
	return "[" + strings.Join(keys, ", ") + "]"
}

var DefaultLevels = Levels{
	{"debug", zapcore.DebugLevel},
	{"info", zapcore.InfoLevel},
	{"warning", zapcore.WarnLevel},
	{"error", zapcore.ErrorLevel},
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FromFlags(flags Flags) Modifier {
	return func(o *options) error {
		cx, cl := DefaultLevels.Find(flags.ConsoleLevel)
		if cl == nil {
			return kflags.NewUsageErrorf("invalid --loglevel-console passed - %s is unknown, valid: %s", flags.ConsoleLevel, DefaultLevels)
		}
		sx, sl := DefaultLevels.Find(flags.SyslogLevel)
		if sl == nil {
			return kflags.NewUsageErrorf("invalid --loglevel-syslog passed - %s is unknown, valid: %s", flags.ConsoleLevel, DefaultLevels)
		}

		cx = max(0, cx-flags.Verbosity)
		sx = max(0, sx-flags.Verbosity)

		o.minConsole = DefaultLevels[cx].Value
		o.minSyslog = DefaultLevels[sx].Value
		return nil
	}
}

func New(name string, mods ...Modifier) (*Logger, error) {
	options := &options{
		minConsole: zap.WarnLevel,
		minSyslog:  zap.InfoLevel,
	}
	if err := Modifiers(mods).Apply(options); err != nil {
		return nil, err
	}

	matchConsole := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= options.minConsole
	})
	matchSyslog := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= options.minSyslog
	})

	console := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	tees := []zapcore.Core{
		zapcore.NewCore(console, zapcore.Lock(os.Stderr), matchConsole),
	}
	for _, level := range []struct {
		Syslog syslog.Priority
		Zap    zapcore.Level
	}{
		{Syslog: syslog.LOG_INFO, Zap: zapcore.InfoLevel},
		{Syslog: syslog.LOG_ERR, Zap: zapcore.ErrorLevel},
		{Syslog: syslog.LOG_WARNING, Zap: zapcore.WarnLevel},
		{Syslog: syslog.LOG_DEBUG, Zap: zapcore.DebugLevel},
	} {
		if !matchSyslog(level.Zap) {
			continue
		}

		logger, err := Syslog(name, level.Syslog, level.Zap)
		if err != nil {
			continue
		}
		tees = append(tees, logger)
	}

	logger := zap.New(zapcore.NewTee(tees...)).Sugar()
	return &Logger{logger}, nil
}
