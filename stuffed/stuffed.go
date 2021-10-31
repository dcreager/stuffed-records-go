// Package stuffed provides a Go implementation of Paul Khuong's stuffed records
// encoding.  This is a modified version of Consistent Overhead Byte Stuffing
// (COBS), which uses the uncommon two-byte sequence `0xfe 0xfd` as the record
// delimiter, instead of the more common one-byte sequence `0x00`.
package stuffed

import (
	"bytes"
	"errors"
	"io"
)

const radix = 0xfd
const maxInitialRun = radix - 1
const maxRemainingRun = (radix * radix) - 1
const delimiterLength = 2
const delimiter0 = 0xfe
const delimiter1 = 0xfd

var (
	// InvalidRunLength is the error that is returned when a stuffed record
	// containing an invalid length prefix.
	InvalidRunLength = errors.New("Invalid run length")
)

// findDelimiter looks for the delimiter sequence within the first maxRun bytes
// of record.  If we find it, we return its index within record.  If not, we
// return the length of the subset of record that we looked in.  (That is, the
// minimum of maxRun and the actual length of record.)
func findDelimiter(record []byte, maxRun int) int {
	if len(record) < maxRun {
		maxRun = len(record)
	} else {
		record = record[:maxRun]
	}
	result := bytes.Index(record, []byte{delimiter0, delimiter1})
	if result == -1 {
		return maxRun
	}
	return result
}

// Encode writes a binary record into an output buffer using the stuffed records
// encoding.  This guarantees that the content that we write does not contain
// any occurrences of the delimiter.  (We do _not_ write a trailing copy of the
// delimiter; it is your responsibility to write this in between records using
// EncodeDelimiter.)
func Encode(record []byte, buf *bytes.Buffer) {
	// For the first run, we encode a maximum of 252 characters, so that we can
	// encode the length in a single byte.
	runSize := findDelimiter(record, maxInitialRun)
	buf.WriteByte(byte(runSize))
	buf.Write(record[:runSize])
	record = record[runSize:]
	if runSize < maxInitialRun {
		// We reached the end (with a virtual terminating delimiter).
		if len(record) == 0 {
			return
		}

		// record should start with delimiter, so skip over it.
		record = record[2:]
	}

	// For any remaining runs, we encode a maximum of 65008 characters, encoding
	// the length in two bytes.
	for {
		runSize := findDelimiter(record, maxRemainingRun)
		buf.WriteByte(byte(runSize % radix))
		buf.WriteByte(byte(runSize / radix))
		buf.Write(record[:runSize])
		record = record[runSize:]
		if runSize < maxRemainingRun {
			// We reached the end (with a virtual terminating delimiter).
			if len(record) == 0 {
				return
			}

			// record should start with delimiter, so skip over it.
			record = record[2:]
		}
	}
}

// EncodeDelimiter writes the stuffed records delimiter to an output buffer.
// You should use this to separate records in your output stream.
func EncodeDelimiter(buf *bytes.Buffer) {
	buf.WriteByte(delimiter0)
	buf.WriteByte(delimiter1)
}

// Decode reads a binary record from an input buffer using the stuffed records
// encoding.  You must ensure that record does not contain any occurrences of
// the delimiter sequence.  (FindDelimiter can help you find the bounds of an
// encoded record before decoding it.)
func Decode(encoded []byte, record *bytes.Buffer) error {
	// For the first run, the length is one byte.
	if len(encoded) < 1 {
		return io.EOF
	}
	runLength := int(encoded[0])
	encoded = encoded[1:]
	if runLength > maxInitialRun {
		return InvalidRunLength
	}

	if len(encoded) < runLength {
		return io.EOF
	}
	record.Write(encoded[:runLength])
	encoded = encoded[runLength:]
	if runLength < maxInitialRun {
		if len(encoded) == 0 {
			return nil
		}
		EncodeDelimiter(record)
	}

	for {
		if len(encoded) < delimiterLength {
			return io.EOF
		}
		runLength := int(encoded[0]) + radix*int(encoded[1])
		encoded = encoded[delimiterLength:]
		if runLength > maxRemainingRun {
			return InvalidRunLength
		}

		if len(encoded) < runLength {
			return io.EOF
		}
		record.Write(encoded[:runLength])
		encoded = encoded[runLength:]
		if runLength < maxRemainingRun {
			if len(encoded) == 0 {
				return nil
			}
			EncodeDelimiter(record)
		}
	}
}

// FindDelimiter returns the index of the first occurrence of the stuffed
// records delimiter in buf, or -1 if it doesn't occur.
func FindDelimiter(record []byte) int {
	return bytes.Index(record, []byte{delimiter0, delimiter1})
}

// FindLastDelimiter returns the index of the lsat occurrence of the stuffed
// records delimiter in buf, or -1 if it doesn't occur.
func FindLastDelimiter(record []byte) int {
	return bytes.LastIndex(record, []byte{delimiter0, delimiter1})
}

// IsStartOfRecord returns whether a particular offset within a buffer is the
// start of a stuffed record.  The offset must either point at the start of the
// buffer, or immediately after a `0xfe 0xfd` delimiter.  This does not check
// that the offset points at a valid record, just that it _could_.  (Decode will
// return an error if it doesn't.)
func IsStartOfRecord(buffer []byte, offset int) bool {
	if offset == 1 || offset >= len(buffer) {
		return false
	}
	if offset >= 2 && (buffer[offset-2] != 0xfe || buffer[offset-1] != 0xfd) {
		return false
	}
	return true
}

// Scanner iterates through a buffer containing zero or more delimited stuffed
// records.
type Scanner struct {
	record []byte
	list   []byte
	buf    bytes.Buffer
}

// Reset updates a Scanner to read from a new buffer of delimited stuffed
// records.
func (s *Scanner) Reset(encodedList []byte) {
	s.record = nil
	s.list = encodedList
	s.buf.Reset()
}

// Next returs whether there is a next stuffed record in the underlying buffer.
// If this returns true, you can use Encoded and Decode to access that record.
func (s *Scanner) Next() bool {
	// Skip over any leading delimiters.
	for bytes.HasPrefix(s.list, []byte{delimiter0, delimiter1}) {
		s.list = s.list[delimiterLength:]
	}

	// If the buffer is now empty, we've reached the end of the list.
	if len(s.list) == 0 {
		return false
	}

	// Otherwise, whatever exists at the start of the buffer, up through the
	// next delimiter, is the next encoded record.
	index := FindDelimiter(s.list)
	if index == -1 {
		s.record = s.list
		s.list = nil
	} else {
		s.record = s.list[:index]
		s.list = s.list[index:]
	}
	return true
}

// Encoded returns the portion of the underlying buffer that contains the
// encoded content of the current stuffed record.
func (s *Scanner) Encoded() []byte {
	return s.record
}

// Decode reads the current stuffed record and decodes it into an output Buffer.
func (s *Scanner) Decode(decoded *bytes.Buffer) error {
	return Decode(s.record, decoded)
}

func checkPrefix(chunk, prefix []byte) (int, int) {
	length := len(chunk)
	if length > len(prefix) {
		length = len(prefix)
	}
	return bytes.Compare(chunk[:length], prefix[:length]), length
}

// CompareEncodedPrefix checks whether the decoded content of a stuffed record
// begins with a prefix, returning 0 if it does.  If it does not, returns -1 or
// 1 depending on whether the decoded content's prefix is less than or greater
// than the desired prefix.  (You provide the _encoded_ stuffed record, and we
// perform the check without decoding the content into a buffer.)
func CompareEncodedPrefix(encoded, prefix []byte) (int, error) {
	// Every byte array starts with the empty byte array.
	if len(prefix) == 0 {
		return 0, nil
	}

	// For the first run, the length is one byte.
	if len(encoded) < 1 {
		return 0, io.EOF
	}
	runLength := int(encoded[0])
	encoded = encoded[1:]
	if runLength > maxInitialRun {
		return 0, InvalidRunLength
	}

	if len(encoded) < runLength {
		return 0, io.EOF
	}
	chunk := encoded[:runLength]
	encoded = encoded[runLength:]
	cmp, consumed := checkPrefix(chunk, prefix)
	if cmp != 0 {
		return cmp, nil
	}
	prefix = prefix[consumed:]

	if runLength < maxInitialRun {
		if len(prefix) == 0 {
			return 0, nil
		}
		if len(encoded) == 0 {
			return -1, nil
		}

		chunk := []byte{0xfe, 0xfd}
		cmp, consumed := checkPrefix(chunk, prefix)
		if cmp != 0 {
			return cmp, nil
		}
		prefix = prefix[consumed:]
	}

	for {
		if len(prefix) == 0 {
			return 0, nil
		}
		if len(encoded) < delimiterLength {
			return 0, io.EOF
		}
		runLength := int(encoded[0]) + radix*int(encoded[1])
		encoded = encoded[delimiterLength:]
		if runLength > maxRemainingRun {
			return 0, InvalidRunLength
		}

		if len(encoded) < runLength {
			return 0, io.EOF
		}
		chunk := encoded[:runLength]
		encoded = encoded[runLength:]
		cmp, consumed := checkPrefix(chunk, prefix)
		if cmp != 0 {
			return cmp, nil
		}
		prefix = prefix[consumed:]

		if runLength < maxRemainingRun {
			if len(prefix) == 0 {
				return 0, nil
			}
			if len(encoded) == 0 {
				return -1, nil
			}

			chunk := []byte{0xfe, 0xfd}
			cmp, consumed := checkPrefix(chunk, prefix)
			if cmp != 0 {
				return cmp, nil
			}
			prefix = prefix[consumed:]
		}
	}
}

// EncodedStartsWith checks whether the decoded content of a stuffed record
// begins with a prefix.  (You provide the _encoded_ stuffed record, and we
// perform the check without decoding the content into a buffer.)
func EncodedStartsWith(encoded, prefix []byte) (bool, error) {
	cmp, err := CompareEncodedPrefix(encoded, prefix)
	return cmp == 0, err
}

// FindRecordsWithPrefix takes a buffer containing a list of stuffed
// records that are sorted by their decoded content, and returns the subset of
// the buffer containing records whose decoded content starts with a particular
// prefix.  We do this without decoding any of the records.
func FindRecordsWithPrefix(encodedList, prefix []byte) ([]byte, error) {
	// min always points at the beginning of an encoded record.  max always
	// points at the end of one.
	min := 0
	max := len(encodedList)
	for bytes.HasPrefix(encodedList[min:max], []byte{delimiter0, delimiter1}) {
		min += delimiterLength
	}
	for bytes.HasSuffix(encodedList[min:max], []byte{delimiter0, delimiter1}) {
		max -= delimiterLength
	}

	end := max
	earliestMatchStart := max
	earliestMatchEnd := min

	// Find the first record that starts with the requested prefix.
	for max > min {
		// Jump to the middle of the remainder of the buffer, then find the
		// start of the enclosing record.
		mid := (max + min) / 2
		index := FindLastDelimiter(encodedList[min:mid])
		recordStart := min
		if index != -1 {
			recordStart += index + delimiterLength
		}

		// Find the end of the record.
		index = FindDelimiter(encodedList[recordStart:max])
		recordEnd := max
		if index != -1 {
			recordEnd = recordStart + index
		}

		// Compare this record to the requested prefix.  If it matches, remember
		// its location, but continue to look for any earlier matching records.
		record := encodedList[recordStart:recordEnd]
		cmp, err := CompareEncodedPrefix(record, prefix)
		if err != nil {
			return nil, err
		}

		switch cmp {
		case -1:
			min = recordEnd
			for bytes.HasPrefix(encodedList[min:max], []byte{delimiter0, delimiter1}) {
				min += delimiterLength
			}
		case 1:
			max = recordStart
			for bytes.HasSuffix(encodedList[min:max], []byte{delimiter0, delimiter1}) {
				max -= delimiterLength
			}
		default:
			earliestMatchStart = recordStart
			earliestMatchEnd = recordEnd
			max = recordStart
			for bytes.HasSuffix(encodedList[min:max], []byte{delimiter0, delimiter1}) {
				max -= delimiterLength
			}
		}
	}

	// If there were no matching records, go ahead and return.
	if earliestMatchStart >= earliestMatchEnd {
		return nil, nil
	}

	// Once the earliest matching record is found, iterate forward until we find
	// the first non-matching record.
	previousRecordEnd := earliestMatchEnd

	// For the first matching record, avoid repeating the prefix check.
	nextRecordStart := previousRecordEnd
	for bytes.HasPrefix(encodedList[nextRecordStart:], []byte{delimiter0, delimiter1}) {
		nextRecordStart += delimiterLength
	}

	// Check the next record to see if it matches the prefix.
	for nextRecordStart < end {
		// Find the end of the record.
		nextRecordEnd := FindDelimiter(encodedList[nextRecordStart:])
		if nextRecordEnd == -1 {
			nextRecordEnd = end
		} else {
			nextRecordEnd += nextRecordStart
		}

		matches, err := EncodedStartsWith(encodedList[nextRecordStart:nextRecordEnd], prefix)
		if err != nil {
			return nil, err
		}

		if !matches {
			// This is the first record that DOESN'T match.  Our result is
			// everything up through the previous record.
			return encodedList[earliestMatchStart:previousRecordEnd], nil
		}

		// This record matches.  Skip past it to find the next record.
		previousRecordEnd = nextRecordEnd
		nextRecordStart = nextRecordEnd
		for bytes.HasPrefix(encodedList[nextRecordStart:], []byte{delimiter0, delimiter1}) {
			nextRecordStart += delimiterLength
		}
	}

	// We made it to the end of the input without finding a record that DOESN'T
	// match.
	return encodedList[earliestMatchStart:previousRecordEnd], nil
}
