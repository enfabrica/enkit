package logger

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Represents a logging priority.
// Logging priorities map to the Debugf, Infof, Warnf, and Errorf printers.
type Priority int

const (
	DebugPriority Priority = iota
	InfoPriority
	WarnPriority
	ErrorPriority
)

// Event represents something that was logged.
type Event struct {
	Time     time.Time
	Priority Priority
	Message  string
}

// Accumulator is a thread safe object implementing the Logger interface that
// accumulates all messages being logged.
//
// You can then use the Retrieve() or Forward() methods to access or print the
// accumulated messages.
type Accumulator struct {
	lock  sync.Mutex
	event []Event
}

func PrintEvent(ev *Event, log Logger) {
	switch ev.Priority {
	case InfoPriority:
		log.Infof("%s", ev.Message)
	case WarnPriority:
		log.Warnf("%s", ev.Message)
	case DebugPriority:
		log.Debugf("%s", ev.Message)
	case ErrorPriority:
		log.Errorf("%s", ev.Message)
	}
}

func (dl *Accumulator) Forward(log Logger) {
	dl.ForwardWithPrinter(log, PrintEvent)
}

func (dl *Accumulator) ForwardWithPrinter(log Logger, printer func(*Event, Logger)) {
	events := dl.Retrieve()
	for _, ev := range events {
		printer(&ev, log)
	}
}

func (dl *Accumulator) Retrieve() []Event {
	dl.lock.Lock()
	defer dl.lock.Unlock()
	events := dl.event
	dl.event = nil
	return events
}

func (dl *Accumulator) Add(prio Priority, format string, args ...interface{}) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	message := fmt.Sprintf(format, args...)
	dl.event = append(dl.event, Event{
		Priority: prio,
		Message:  message,
		Time:     time.Now(),
	})
}
func (dl *Accumulator) Debugf(format string, args ...interface{}) {
	dl.Add(DebugPriority, format, args...)
}
func (dl *Accumulator) Infof(format string, args ...interface{}) {
	dl.Add(InfoPriority, format, args...)
}
func (dl *Accumulator) Errorf(format string, args ...interface{}) {
	dl.Add(ErrorPriority, format, args...)
}
func (dl *Accumulator) Warnf(format string, args ...interface{}) {
	dl.Add(WarnPriority, format, args...)
}
func (dl *Accumulator) SetOutput(writer io.Writer) {
}

func NewAccumulator() *Accumulator {
	return &Accumulator{}
}
