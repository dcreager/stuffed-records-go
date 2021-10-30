package stuffed

import (
	"bytes"
)

// RecordBuilder makes it easier to build up the content of individual records,
// which are then written into a buffer using the stuffed records encoding.  To
// build up the content of an individual record, just use the RecordBuilder as a
// bytes.Buffer.  Once a record is done, call FinishRecord.  Once you are done
// with all records, call Encode to get the encoded representation of
// everything.
type RecordBuilder struct {
	bytes.Buffer
	start         int
	recordIndices []index
}

type index struct {
	start, end int
}

// FinishRecord indicates that you have finished constructing an individual
// record.  We don't actually encode the record until you call Encode, when we
// encode _all_ of the records that you add to the builder.
func (rb *RecordBuilder) FinishRecord() {
	end := rb.Len()
	rb.recordIndices = append(rb.recordIndices, index{rb.start, end})
	rb.start = end
}

// Encode encodes all of the records in this builder into an output buffer,
// using the stuffed records encoding.
func (rb *RecordBuilder) Encode(dest *bytes.Buffer) {
	records := rb.Bytes()
	for _, index := range rb.recordIndices {
		record := records[index.start:index.end]
		Encode(record, dest)
		EncodeDelimiter(dest)
	}
}
