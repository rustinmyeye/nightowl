package logger

import (
	//"io"
	//logg "log"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

const (
	// queueSize is the size of the logger queue
	queueSize = 10000
)

var (
	// logger is the package level logger.
	log *logger
)

// logger handles writing LogMessages to the supplied io.Writer.
type logger struct {
	out       io.Writer
	formatter Formatter
	defaults  Fields
	verbosity int
	queue     chan *Message
	mutex     sync.RWMutex
}

func init() {
	// This is a temporary fix for a bug on the Cisco redfish API
	// Unsolicited response received on idle HTTP channel starting with "\n"; err=<nil>
	//logg.SetOutput(io.Discard)

	var formatter Formatter
	formatter = NewJSONFormatter()

	log = &logger{
		out:       os.Stdout,
		formatter: formatter,
		queue:     make(chan *Message, queueSize),
		verbosity: 1,
	}
	go log.start()
}

func Debugf(verbosity int, format string, args ...interface{}) {
	newMessage().Debugf(verbosity, format, args...)
}

func Infof(code int, format string, args ...interface{}) {
	newMessage().Infof(code, format, args...)
}

func Warnf(code int, format string, args ...interface{}) {
	newMessage().Warnf(code, format, args...)
}

func SetDefaults(f Fields) {
	log.SetDefaults(f)
}

func SetVerbosity(v int) {
	log.SetVerbosity(v)
}

func ParseLevel(l string) (int, error) {
	switch l{
	case "warn":
		return 0, nil
	case "info":
		return 1, nil
	case "debug":
		return 2, nil
	default:
		return -1, errors.New("log level unknown")
	}
}

// Flush returns when all messages in the queue prior to the Flush log have
// been written.
func (l *logger) Flush() {
	// Add a very brief delay to ensure nearly-concurrent messages have been
	// received prior to being flushed
	time.Sleep(100 * time.Millisecond)
	flushed := make(chan struct{}, 1)
	l.Log(&Message{
		kind:    kindFlush,
		flushed: flushed,
	})
	<-flushed
}

// Log queues the Message for writing.
func (l *logger) Log(m *Message) {
	// queue the message
	l.queue <- m

	if m.code == 0 || m.kind != kindInfo {
		return
	}
}

func WithError(err error) *Message {
	return newMessage().WithError(err)
}

func WithFields(fields Fields) *Message {
	return newMessage().WithFields(fields)
}

func (l *logger) SetDefaults(f Fields) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.defaults = f
}

func (l *logger) SetVerbosity(v int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.verbosity = v
}

// start the process that consumes the queued messages, formats them, and
// then writes them to io.Writer.
func (l *logger) start() {
	for {
		if msg, ok := <-l.queue; ok {
			l.write(msg)
			if msg.flushed != nil {
				close(msg.flushed)
			}
		} else {
			return
		}
	}
}

func (l *logger) write(msg *Message) {
	// flush messages are ignored
	if msg.kind == kindFlush {
		return
	}

	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// excluded by verbosity
	if msg.kind == kindDebug && msg.verbosity > l.verbosity {
		return
	}

	// set default fields, format, and write
	bytes := l.formatter.Format(msg.WithFields(l.defaults))
	_, _ = l.out.Write(bytes)
}
