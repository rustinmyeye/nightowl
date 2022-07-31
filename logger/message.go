package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// kindWarn denotes an Warn message.
	kindWarn = iota
	// kindInfo denotes an Info message.
	kindInfo
	// kindDebug denotes a Debug message
	kindDebug
	// kindFlush denotes a Flush message.
	kindFlush
)

// newMessage constructs a default Message.
func newMessage() *Message {
	msg := Message{
		when: time.Now(),
	}

	_, file, line, ok := runtime.Caller(2)
	if ok {
		msg.WithSource(
			filepath.Base(filepath.Dir(file)),
			filepath.Base(file),
			line,
		)
	}

	return &msg
}

// Message tracks an individual log message to write.
type Message struct {
	kind      int
	when      time.Time
	code      int
	verbosity int
	src       string
	fields    Fields
	message   string
	flushed   chan struct{}
}

// WithError is a helper method to ensure all errors included in log messages
// use the same key, "error".
func (m *Message) WithError(err error) *Message {
	if err == nil {
		return m
	}

	return m.WithFields(Fields{Error: err})
}

// WithFields adds fields to the Message.
func (m *Message) WithFields(f Fields) *Message {
	if m.fields == nil {
		m.fields = f
		return m
	}

	// Merge in any existing fields as necessary
	for k, v := range f {
		m.fields[k] = v
	}

	return m
}

// WithSource allows the caller to manually set the source for this log message
func (m *Message) WithSource(pkg, file string, line int) *Message {
	m.src = fmt.Sprintf("%s/%s:%d", pkg, file, line)
	return m
}

// Debugf sends the message to the logger at the given verbosity.
func (m *Message) Debugf(verbosity int, msg string, args ...interface{}) {
	m.kind = kindDebug
	m.verbosity = verbosity
	if len(args) > 0 {
		m.message = fmt.Sprintf(msg, args...)
	} else {
		m.message = msg
	}
	log.Log(m)
}

// Infof sends the message to the logger with the code specified.
func (m *Message) Infof(code int, msg string, args ...interface{}) {
	m.kind = kindInfo
	m.code = code
	if len(args) > 0 {
		m.message = fmt.Sprintf(msg, args...)
	} else {
		m.message = msg
	}
	log.Log(m)
}

// Warnf sends the message to the logger with the code specified.
func (m *Message) Warnf(code int, msg string, args ...interface{}) {
	m.kind = kindWarn
	m.code = code
	if len(args) > 0 {
		m.message = fmt.Sprintf(msg, args...)
	} else {
		m.message = msg
	}
	log.Log(m)
}
