package stuffed_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dcreager/stuffed-records-go/stuffed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const string32 = "abcdefghijklmnopqrstuvwxyz012345"
const string64 = string32 + string32
const string128 = string64 + string64
const string256 = string128 + string128

type shortTestCase struct {
	decoded string
	encoded string
}

var shortTestCases = []shortTestCase{
	{"", "\x00"},
	{"abc", "\x03abc"},
	{"\xfe\xfd", "\x00\x00\x00"},
	{"abc\xfe\xfd", "\x03abc\x00\x00"},
	{"\xfe\xfdabc", "\x00\x03\x00abc"},
	{"abc\xfe\xfdabc", "\x03abc\x03\x00abc"},
	{string128, "\x80" + string128},
	{string256, "\xfc" + string256[0:252] + "\x04\x00" + string256[252:]},
	{
		strings.Repeat("a", 64008),
		"\xfc" + strings.Repeat("a", 252) + "\x00\xfc" + strings.Repeat("a", 63756),
	},
	{
		strings.Repeat("a", 64008) + "\xfe\xfd",
		"\xfc" + strings.Repeat("a", 252) + "\x00\xfc" + strings.Repeat("a", 63756) + "\x00\x00",
	},
}

func TestEncodeRecords(t *testing.T) {
	for _, tc := range shortTestCases {
		var buf bytes.Buffer
		stuffed.Encode([]byte(tc.decoded), &buf)
		assert.Equal(t, buf.String(), string(tc.encoded))
	}
}

func TestDecodeRecords(t *testing.T) {
	for _, tc := range shortTestCases {
		var buf bytes.Buffer
		err := stuffed.Decode([]byte(tc.encoded), &buf)
		require.NoError(t, err)
		assert.Equal(t, buf.String(), string(tc.decoded))
	}

	shortRecords := []string{
		"\x01",
		"\x03",
		"\x03a",
		"\x03ab",
		"\x03abc\x01",
	}
	for _, encoded := range shortRecords {
		var buf bytes.Buffer
		err := stuffed.Decode([]byte(encoded), &buf)
		assert.Equal(t, io.EOF, err)
	}

	invalidRecords := []string{
		"\xfd",
		"\xfe",
		"\xff",
		"\xfc" + strings.Repeat("a", 252) + "\xff\xff",
		"\xfc" + strings.Repeat("a", 252) + "\xfe\xff",
		"\xfc" + strings.Repeat("a", 252) + "\xff\xfe",
		"\xfc" + strings.Repeat("a", 252) + "\xff\xfd",
		"\xfc" + strings.Repeat("a", 252) + "\xfe\xfd",
	}
	for _, encoded := range invalidRecords {
		var buf bytes.Buffer
		err := stuffed.Decode([]byte(encoded), &buf)
		assert.Equal(t, stuffed.InvalidRunLength, err)
	}
}
