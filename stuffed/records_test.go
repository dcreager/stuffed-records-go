package stuffed_test

import (
	"bytes"
	"testing"

	"github.com/dcreager/stuffed-records-go/stuffed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkRecordBuilder(t require.TestingT, inputList []string) {
	var builder stuffed.RecordBuilder
	var encoded bytes.Buffer
	for _, str := range inputList {
		builder.WriteString(str)
		builder.FinishRecord()
	}
	builder.Encode(&encoded)

	var decoded bytes.Buffer
	var scanner stuffed.Scanner
	scanner.Reset(encoded.Bytes())
	actual := []string{}
	for scanner.Next() {
		decoded.Reset()
		err := scanner.Decode(&decoded)
		require.NoError(t, err)
		actual = append(actual, decoded.String())
	}
	assert.Equal(t, inputList, actual)
}

func TestRecordBuilder(t *testing.T) {
	testCases := [][]string{
		{},
		{"hello", "there"},
		{"what is\xfe\xfdgoing on"},
	}
	for i := range testCases {
		checkRecordBuilder(t, testCases[i])
	}
}
