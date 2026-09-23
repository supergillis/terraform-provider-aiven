package clickhouse

import (
	"bytes"
	"fmt"
)

// Escape quotes and escapes a ClickHouse identifier for use in a statement.
func Escape(identifier string) string {
	return quote([]byte(identifier), '`')
}

// QuoteString quotes and escapes a ClickHouse string literal for use in a statement.
func QuoteString(value string) string {
	return quote([]byte(value), '\'')
}

func quote(value []byte, delimiter byte) string {
	buf := new(bytes.Buffer)
	buf.WriteByte(delimiter)

	for _, b := range value {
		switch b {
		case 0:
			buf.WriteString("\\0")
		case '\b':
			buf.WriteString("\\b")
		case '\f':
			buf.WriteString("\\f")
		case '\r':
			buf.WriteString("\\r")
		case '\n':
			buf.WriteString("\\n")
		case '\t':
			buf.WriteString("\\t")
		case delimiter, '\\':
			buf.WriteByte('\\')
			buf.WriteByte(b)
		default:
			if b < 0x20 || b > 0x7e {
				fmt.Fprintf(buf, "\\x%02x", b)
			} else {
				buf.WriteByte(b)
			}
		}
	}

	buf.WriteByte(delimiter)
	return buf.String()
}
