package logger

const (
	// JSON is JSON format.
	JSON = "json"
)

// Formatter is responsible for formatting logger Messages.
type Formatter interface {
	Format(m *Message) []byte
}
