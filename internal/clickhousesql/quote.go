package clickhousesql

import (
	"bytes"
	"fmt"
)

// QuoteIdentifier quotes and escapes a ClickHouse identifier.
func QuoteIdentifier(identifier string) string {
	return quote([]byte(identifier), '`')
}

// QuoteString quotes and escapes a ClickHouse string literal.
func QuoteString(value string) string {
	return quote([]byte(value), '\'')
}

func quote(value []byte, delimiter byte) string {
	escapeMap := map[byte]string{
		0:    "\\0",
		'\b': "\\b",
		'\f': "\\f",
		'\r': "\\r",
		'\n': "\\n",
		'\t': "\\t",
		'\\': "\\\\",
	}
	escapeMap[delimiter] = "\\" + string(delimiter)
	buffer := new(bytes.Buffer)
	buffer.WriteByte(delimiter)

	for _, character := range value {
		escaped, exists := escapeMap[character]
		switch {
		case exists:
			buffer.WriteString(escaped)
		case character < 0x20 || character > 0x7e:
			buffer.WriteString(fmt.Sprintf("\\x%02x", character))
		default:
			buffer.WriteByte(character)
		}
	}

	buffer.WriteByte(delimiter)
	return buffer.String()
}
