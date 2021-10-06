package stuffed_test

import (
	"bytes"
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
