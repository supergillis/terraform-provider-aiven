package clickhousesql

import (
	"bytes"
	"fmt"
)

// SystemDatabase runs statements that target no particular database, such as CREATE ROLE or ALTER USER.
const SystemDatabase = "system"

// QuoteIdentifier quotes and escapes a ClickHouse identifier.
func QuoteIdentifier(identifier string) string {
	return quote([]byte(identifier), '`')
}

// QuoteString quotes and escapes a ClickHouse string literal.
func QuoteString(value string) string {
	return quote([]byte(value), '\'')
}

func quote(value []byte, delimiter byte) string {
	buffer := new(bytes.Buffer)
	buffer.WriteByte(delimiter)

	for _, character := range value {
		switch character {
		case 0:
			buffer.WriteString("\\0")
		case '\b':
			buffer.WriteString("\\b")
		case '\f':
			buffer.WriteString("\\f")
		case '\r':
			buffer.WriteString("\\r")
		case '\n':
			buffer.WriteString("\\n")
		case '\t':
			buffer.WriteString("\\t")
		case delimiter, '\\':
			buffer.WriteByte('\\')
			buffer.WriteByte(character)
		default:
			if character < 0x20 || character > 0x7e {
				fmt.Fprintf(buffer, "\\x%02x", character)
			} else {
				buffer.WriteByte(character)
			}
		}
	}

	buffer.WriteByte(delimiter)
	return buffer.String()
}
