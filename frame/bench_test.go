package frame

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func BenchmarkDecodeBodyRaw(b *testing.B) {
	body := bytes.Repeat([]byte{1, 2, 3, 4}, 256)
	header := Header{ZipSize: uint16(len(body)), UnzipSize: uint16(len(body))}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBody(header, body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeBodyZlib(b *testing.B) {
	body := bytes.Repeat([]byte{1, 2, 3, 4}, 256)
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(body); err != nil {
		b.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		b.Fatal(err)
	}
	raw := compressed.Bytes()
	header := Header{ZipSize: uint16(len(raw)), UnzipSize: uint16(len(body))}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBody(header, raw); err != nil {
			b.Fatal(err)
		}
	}
}
