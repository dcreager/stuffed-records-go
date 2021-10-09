package stuffed_test

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/dcreager/stuffed-records-go/stuffed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var delimiter = []byte{0xfe, 0xfd}

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

func shortTestCaseInputs() []string {
	var result []string
	for _, tc := range shortTestCases {
		result = append(result, tc.decoded)
	}
	return result
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

func ExampleScanner() {
	encoded := []byte("\x03abc\xfe\xfd\x00\xfe\xfd\xfe\xfd\x041234\xfe\xfd")
	var s stuffed.Scanner
	var decoded bytes.Buffer
	s.Reset(encoded)
	for s.Next() {
		decoded.Reset()
		err := s.Decode(&decoded)
		if err != nil {
			panic(err)
		}
		fmt.Println(decoded.String())
	}
	// Output:
	// abc
	//
	// 1234
}

func parseStrings(encoded []byte) ([]string, error) {
	decodedList := []string{}
	var s stuffed.Scanner
	s.Reset(encoded)
	for s.Next() {
		var decoded bytes.Buffer
		err := s.Decode(&decoded)
		if err != nil {
			return nil, err
		}
		decodedList = append(decodedList, decoded.String())
	}
	return decodedList, nil
}

func checkListRoundTrip(t require.TestingT, inputList []string) {
	var buf bytes.Buffer
	for _, input := range inputList {
		stuffed.EncodeDelimiter(&buf)
		stuffed.Encode([]byte(input), &buf)
	}
	stuffed.EncodeDelimiter(&buf)
	decodedList, err := parseStrings(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, inputList, decodedList)
}

func TestRoundTripList(t *testing.T) {
	checkListRoundTrip(t, shortTestCaseInputs())
}

type prefixTestCase struct {
	prefix   string
	expected []string
}

var prefixTestCases = []prefixTestCase{
	{"", shortTestCaseInputs()},
	{
		"abc",
		[]string{
			"abc",
			"abc\xfe\xfd",
			"abc\xfe\xfdabc",
			string128,
			string256,
		},
	},
	{
		"abc\xfe",
		[]string{
			"abc\xfe\xfd",
			"abc\xfe\xfdabc",
		},
	},
	{
		"abc\xfe\xfd",
		[]string{
			"abc\xfe\xfd",
			"abc\xfe\xfdabc",
		},
	},
}

func checkEncodedStartsWith(t require.TestingT, inputList []string, prefix string, expected []string) {
	var encoded bytes.Buffer
	for _, input := range inputList {
		stuffed.EncodeDelimiter(&encoded)
		stuffed.Encode([]byte(input), &encoded)
	}
	stuffed.EncodeDelimiter(&encoded)

	var actual []string
	var s stuffed.Scanner
	s.Reset(encoded.Bytes())
	for s.Next() {
		matches, err := stuffed.EncodedStartsWith(s.Encoded(), []byte(prefix))
		require.NoError(t, err)
		if matches {
			var decoded bytes.Buffer
			err := s.Decode(&decoded)
			require.NoError(t, err)
			actual = append(actual, decoded.String())
		}
	}

	assert.Equal(t, expected, actual)
}

func TestEncodedStartsWith(t *testing.T) {
	for _, tc := range prefixTestCases {
		checkEncodedStartsWith(t, shortTestCaseInputs(), tc.prefix, tc.expected)
	}
}

func checkFindRecordsWithPrefix(t require.TestingT, inputList []string, prefix string, expected []string) {
	sort.Strings(inputList)
	sort.Strings(expected)

	var encoded bytes.Buffer
	for _, input := range inputList {
		stuffed.EncodeDelimiter(&encoded)
		stuffed.Encode([]byte(input), &encoded)
	}
	stuffed.EncodeDelimiter(&encoded)

	matching, err := stuffed.FindRecordsWithPrefix(encoded.Bytes(), []byte(prefix))
	require.NoError(t, err)
	assert.False(t, bytes.HasPrefix(matching, delimiter))
	assert.False(t, bytes.HasSuffix(matching, delimiter))

	var actual []string
	var s stuffed.Scanner
	s.Reset(matching)
	for s.Next() {
		var decoded bytes.Buffer
		err := s.Decode(&decoded)
		require.NoError(t, err)
		actual = append(actual, decoded.String())
	}

	assert.Equal(t, expected, actual)
}

func TestFindRecordsWithPrefix(t *testing.T) {
	for _, tc := range prefixTestCases {
		checkFindRecordsWithPrefix(t, shortTestCaseInputs(), tc.prefix, tc.expected)
	}
}
