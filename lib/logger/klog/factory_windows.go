// +build windows

package klog

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"strings"
)

type Logger struct {
	*zap.SugaredLogger
}

func (l *Logger) SetOutput(writer io.Writer) {
}

type Flags struct {
	ConsoleLevel string
	Verbosity    int
	Quiet        bool
}

func DefaultFlags() *Flags {
	return &Flags{
		ConsoleLevel: "warn",
		Verbosity:    0,
	}
}

func (cf *Flags) Register(flags kflags.FlagSet, prefix string) *Flags {
	flags.StringVar(&cf.ConsoleLevel, prefix+"loglevel-console", cf.ConsoleLevel, "Can be debug, info, warn, error. Indicates the minimum severity of messages to log on the console")
	flags.IntVar(&cf.Verbosity, prefix+"verbosity", cf.Verbosity, "Increases the verbosity level of logs by the specified amount")
	flags.BoolVar(&cf.Quiet, prefix+"quiet", cf.Quiet, "If set to true, only errors will be logged on the console")
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

		cx = max(0, cx-flags.Verbosity)
		o.minConsole = DefaultLevels[cx].Value
		if flags.Quiet {
			o.minConsole = zapcore.ErrorLevel
		}
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

	console := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	tees := []zapcore.Core{
		zapcore.NewCore(console, zapcore.Lock(os.Stderr), matchConsole),
	}

	logger := zap.New(zapcore.NewTee(tees...)).Sugar()
	return &Logger{logger}, nil
}
