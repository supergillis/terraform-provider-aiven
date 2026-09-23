package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscape(t *testing.T) {
	testdata := []struct {
		in  string
		out string
	}{
		{
			in:  "O`sullivan",
			out: "`O\\`sullivan`",
		},
		{
			in:  "simple",
			out: "`simple`",
		},
		{
			in:  "random \x00 null byte",
			out: "`random \\0 null byte`",
		},
		{
			in:  "\xa3",
			out: "`\\xa3`",
		},
		{
			in: "😀",
			// GRINNING FACE is 0xF0 0x9F 0x98 0x80 in UTF 8
			out: "`\\xf0\\x9f\\x98\\x80`",
		},
	}

	for _, test := range testdata {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, test.out, Escape(test.in))
		})
	}
}

func TestQuoteString(t *testing.T) {
	testdata := []struct {
		in  string
		out string
	}{
		{
			in:  "it's",
			out: `'it\'s'`,
		},
		{
			in:  "simple",
			out: "'simple'",
		},
		{
			in:  `back\slash`,
			out: `'back\\slash'`,
		},
		{
			in:  "random \x00 null byte",
			out: `'random \0 null byte'`,
		},
	}

	for _, test := range testdata {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, test.out, QuoteString(test.in))
		})
	}
}
