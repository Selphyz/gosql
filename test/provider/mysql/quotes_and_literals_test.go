package mysql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/provider/mysql"
)

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple identifier",
			input:    "users",
			expected: "`users`",
		},
		{
			name:     "identifier with special char",
			input:    "user_data",
			expected: "`user_data`",
		},
		{
			name:     "identifier with backtick",
			input:    "user`table",
			expected: "`user``table`",
		},
		{
			name:     "identifier with multiple backticks",
			input:    "user`data`test",
			expected: "`user``data``test`",
		},
		{
			name:     "reserved keyword",
			input:    "select",
			expected: "`select`",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "``",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			result := prov.QuoteIdent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteLiteral_Nil(t *testing.T) {
	prov := &mysql.Provider{}
	result, err := prov.QuoteLiteral(nil)
	require.NoError(t, err)
	assert.Equal(t, "NULL", result)
}

func TestQuoteLiteral_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it\\'s'",
		},
		{
			name:     "string with backslash",
			input:    "path\\to\\file",
			expected: "'path\\\\to\\\\file'",
		},
		{
			name:     "string with null byte",
			input:    "test\x00null",
			expected: "'test\\0null'",
		},
		{
			name:     "string with newline",
			input:    "line1\nline2",
			expected: "'line1\\nline2'",
		},
		{
			name:     "string with carriage return",
			input:    "line1\rline2",
			expected: "'line1\\rline2'",
		},
		{
			name:     "string with substitute char",
			input:    "test\x1aend",
			expected: "'test\\Zend'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			result, err := prov.QuoteLiteral(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteLiteral_Bytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "valid UTF-8 bytes",
			input:    []byte("hello"),
			expected: "'hello'",
		},
		{
			name:     "nil bytes",
			input:    nil,
			expected: "NULL",
		},
		{
			name:     "invalid UTF-8 bytes",
			input:    []byte{0xFF, 0xFE},
			expected: "0xfffe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			result, err := prov.QuoteLiteral(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteLiteral_Bool(t *testing.T) {
	prov := &mysql.Provider{}

	result, err := prov.QuoteLiteral(true)
	require.NoError(t, err)
	assert.Equal(t, "1", result)

	result, err = prov.QuoteLiteral(false)
	require.NoError(t, err)
	assert.Equal(t, "0", result)
}

func TestQuoteLiteral_Integer(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "int",
			input:    int(42),
			expected: "42",
		},
		{
			name:     "int8",
			input:    int8(-1),
			expected: "-1",
		},
		{
			name:     "int16",
			input:    int16(1000),
			expected: "1000",
		},
		{
			name:     "int32",
			input:    int32(100000),
			expected: "100000",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "uint",
			input:    uint(42),
			expected: "42",
		},
		{
			name:     "uint8",
			input:    uint8(255),
			expected: "255",
		},
		{
			name:     "uint16",
			input:    uint16(65535),
			expected: "65535",
		},
		{
			name:     "uint32",
			input:    uint32(4294967295),
			expected: "4294967295",
		},
		{
			name:     "uint64",
			input:    uint64(18446744073709551615),
			expected: "18446744073709551615",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			result, err := prov.QuoteLiteral(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteLiteral_Float(t *testing.T) {
	prov := &mysql.Provider{}

	result, err := prov.QuoteLiteral(float32(3.14))
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	result, err = prov.QuoteLiteral(float64(2.718))
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestQuoteLiteral_Time(t *testing.T) {
	prov := &mysql.Provider{}

	ts := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	result, err := prov.QuoteLiteral(ts)
	require.NoError(t, err)
	assert.Contains(t, result, "2024-01-15")
	assert.Contains(t, result, "10:30:45")
	assert.True(t, result[0] == '\'')
	assert.True(t, result[len(result)-1] == '\'')
}

func TestQuoteLiteral_Stringer(t *testing.T) {
	prov := &mysql.Provider{}

	// Create a custom stringer
	customStr := CustomStringer{value: "custom_value"}
	result, err := prov.QuoteLiteral(customStr)
	require.NoError(t, err)
	assert.Equal(t, "'custom_value'", result)
}

func TestQuoteLiteral_UnsupportedType(t *testing.T) {
	prov := &mysql.Provider{}

	// Unsupported types should return an error
	result, err := prov.QuoteLiteral(map[string]string{"key": "value"})
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "unsupported literal type")
}

type CustomStringer struct {
	value string
}

func (cs CustomStringer) String() string {
	return cs.value
}