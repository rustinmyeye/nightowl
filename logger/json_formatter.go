package logger

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// NewJSONFormatter builds a JSON compatible writer.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		encoder: jsoniter.ConfigCompatibleWithStandardLibrary,
	}
}

// JSONFormatter formats a Message into JSON.
type JSONFormatter struct {
	encoder jsoniter.API
}

func (jf *JSONFormatter) Format(m *Message) []byte {
	payload := make(map[string]interface{})

	payload["ts"] = m.when.Format(time.RFC3339)

	switch m.kind {
	case kindInfo:
		payload["level"] = "INFO"
		payload["code"] = fmt.Sprintf("%04d", m.code)
	case kindWarn:
		payload["level"] = "WARN"
		payload["code"] = fmt.Sprintf("%04d", m.code)
	case kindDebug:
		payload["level"] = "DEBUG"
	}

	if m.src != "" {
		payload["src"] = m.src
	}

	payload["m"] = m.message

	for k, v := range m.fields {
		if k == "m" || k == "src" || k == "code" {
			continue
		}

		if k == Error {
			if err, ok := v.(error); ok {
				v = err.Error()
			}
		}

		payload[k] = v
	}

	doc, err := jf.encoder.Marshal(payload)
	if err != nil {
		WithError(err).Infof(101, "unable to marshal logger payload")
		return nil
	}

	return append(doc, byte('\n'))
}
