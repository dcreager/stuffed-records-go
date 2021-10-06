// Package stuffed provides a Go implementation of Paul Khuong's stuffed records
// encoding.  This is a modified version of Consistent Overhead Byte Stuffing
// (COBS), which uses the uncommon two-byte sequence `0xfe 0xfd` as the record
// delimiter, instead of the more common one-byte sequence `0x00`.
package stuffed
