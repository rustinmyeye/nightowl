package logger

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// Code is the field key containing the error/info code.
	Code = "code"
	// Error is the field key the error message.
	Error = "err"
	// Hostname is the field key for hostname.
	Hostname = "host"
	// Duration is the field key containing the execution duration
	Duration = "dur"
	// RemoteAddr is the field key containing HTTP remote address.
	RemoteAddr = "remoteAddr"
	// Source is the field key containing the source.
	Source = "src"
	// URL is the field key containing the URL.
	URL = "url"
)

// Fields is a collection of key/value pairs to include with a log message.
type Fields map[string]interface{}

// StringValue returns a field's value as a string.
func (f Fields) StringValue(k string) string {
	if v, ok := f[k]; ok {
		return toString(v)
	}
	return ""
}

// toString encodes a field as a string
func toString(v interface{}) string {
	switch vt := v.(type) {
	case string:
		return encodeValue(vt)
	case error:
		return encodeValue(vt.Error())
	case int:
		return strconv.FormatInt(int64(vt), 10)
	case int8:
		return strconv.FormatInt(int64(vt), 10)
	case int16:
		return strconv.FormatInt(int64(vt), 10)
	case int32:
		return strconv.FormatInt(int64(vt), 10)
	case int64:
		return strconv.FormatInt(vt, 10)
	case uint:
		return strconv.FormatUint(uint64(vt), 10)
	case uint8:
		return strconv.FormatUint(uint64(vt), 10)
	case uint16:
		return strconv.FormatUint(uint64(vt), 10)
	case uint32:
		return strconv.FormatUint(uint64(vt), 10)
	case uint64:
		return strconv.FormatUint(vt, 10)
	case float32:
		return strconv.FormatFloat(float64(vt), 'f', 5, 32)
	case float64:
		return strconv.FormatFloat(vt, 'f', 5, 64)
	case bool:
		return strconv.FormatBool(vt)
	case time.Time:
		return vt.Format(time.RFC3339)
	case fmt.Stringer:
		return encodeValue(vt.String())
	default:
		return encodeValue(fmt.Sprintf("%v", vt))
	}
}

// encodeValue wraps a string in quotes if necessary.
func encodeValue(s string) string {
	if strings.ContainsRune(s, ' ') {
		return strconv.Quote(s)
	}
	return s
}
