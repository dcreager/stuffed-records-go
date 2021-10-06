# Stuffed Records

This is a Go implementation of Paul Khuong's [stuffed records][] encoding.  This
is a modified version of [Consistent Overhead Byte Stuffing][cobs] (COBS), which
uses the uncommon two-byte sequence `0xfe 0xfd` as the record delimiter, instead
of the more common one-byte sequence `0x00`.

[stuffed records]: http://pvk.ca/Blog/2021/01/11/stuff-your-logs/
[COBS]: https://en.wikipedia.org/wiki/Consistent_Overhead_Byte_Stuffing
