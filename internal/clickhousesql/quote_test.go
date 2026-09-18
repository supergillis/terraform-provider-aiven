package clickhousesql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuoteIdentifier(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{input: "O`sullivan", expected: "`O\\`sullivan`"},
		{input: "O'sullivan", expected: "`O'sullivan`"},
		{input: "simple", expected: "`simple`"},
		{input: "random \x00 null byte", expected: "`random \\0 null byte`"},
		{input: string([]byte{0xA3}), expected: "`\\xa3`"},
		{input: "😀", expected: "`\\xf0\\x9f\\x98\\x80`"},
	}

	for _, testCase := range testCases {
		require.Equal(t, testCase.expected, QuoteIdentifier(testCase.input))
	}
}

func TestQuoteString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "'can\\'t `this` \\\\ change'", QuoteString("can't `this` \\ change"))
}
