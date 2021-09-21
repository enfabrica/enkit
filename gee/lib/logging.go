package lib

import (
  "github.com/xo/terminfo"
)

type Logger struct {
  ti *terminfo.Terminfo
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

func (logger *Logger) Banner(lines []string) {
}



