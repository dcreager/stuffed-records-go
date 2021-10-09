package stuffed_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/dcreager/stuffed-records-go/stuffed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

const radix = 0xfd
const maxInitialRun = radix - 1
const maxRemainingRun = (radix * radix) - 1

type largeChunkContent struct{}

func (largeChunkContent) Content() string {
	return strings.Repeat("a", maxRemainingRun)
}

func (largeChunkContent) String() string {
	return "[large chunk]"
}

var inputString = rapid.Custom(func(t *rapid.T) string {
	smallChunk := rapid.String()
	largeChunk := rapid.Just(largeChunkContent{})
	delimiter := rapid.Just("\xfe\xfd")
	generator := rapid.SliceOf(rapid.OneOf(smallChunk, largeChunk, delimiter))
	chunks := generator.Draw(t, "chunks").([]interface{})
	var buf bytes.Buffer
	for _, chunk := range chunks {
		large, ok := chunk.(largeChunkContent)
		if ok {
			buf.WriteString(large.Content())
		} else {
			buf.WriteString(chunk.(string))
		}
	}
	return buf.String()
})

func TestRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := inputString.Draw(t, "input").(string)
		var encoded bytes.Buffer
		stuffed.Encode([]byte(input), &encoded)
		var decoded bytes.Buffer
		err := stuffed.Decode(encoded.Bytes(), &decoded)
		require.NoError(t, err)
		assert.Equal(t, input, decoded.String())
	})
}

func TestRoundTripRandomLists(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		inputList := rapid.SliceOf(inputString).Draw(t, "inputList").([]string)
		checkListRoundTrip(t, inputList)
	})
}

func TestEncodedStartsWithRandomLists(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		prefix := inputString.Draw(t, "prefix").(string)
		inputList := rapid.SliceOf(inputString).Draw(t, "inputList").([]string)
		shouldBePrefixed := reflect.ValueOf(rapid.ArrayOf(len(inputList), rapid.Bool()).Draw(t, "shouldBePrefixed"))

		// Ensure that every "prefixed" input actually starts with the prefix.
		for i := range inputList {
			if shouldBePrefixed.Index(i).Bool() {
				inputList[i] = prefix + inputList[i]
			}
		}

		// Create our expected result list, by determining which inputs are
		// prefixed.  (Some of the "other" inputs might coincidentally start
		// with the chosen prefix.)
		var expected []string
		for i := range inputList {
			if strings.HasPrefix(inputList[i], prefix) {
				expected = append(expected, inputList[i])
			}
		}

		checkEncodedStartsWith(t, inputList, prefix, expected)
	})
}
