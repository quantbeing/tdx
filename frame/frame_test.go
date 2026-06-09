package frame

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
)

func TestParseHeaderAndDecodeUncompressedBody(t *testing.T) {
	headerBytes := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(headerBytes[0:4], 0x756bceb1)
	binary.LittleEndian.PutUint32(headerBytes[4:8], 0x6408010c)
	binary.LittleEndian.PutUint32(headerBytes[8:12], 0x052d0000)
	binary.LittleEndian.PutUint16(headerBytes[12:14], 3)
	binary.LittleEndian.PutUint16(headerBytes[14:16], 3)

	header, err := ParseHeader(headerBytes)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	body, err := DecodeBody(header, []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("DecodeBody: %v", err)
	}
	if !bytes.Equal(body, []byte{1, 2, 3}) {
		t.Fatalf("body = %v, want [1 2 3]", body)
	}
}

func TestDecodeCompressedBody(t *testing.T) {
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write([]byte("tdx-body"))
	_ = zw.Close()

	header := Header{ZipSize: uint16(compressed.Len()), UnzipSize: uint16(len("tdx-body"))}
	body, err := DecodeBody(header, compressed.Bytes())
	if err != nil {
		t.Fatalf("DecodeBody compressed: %v", err)
	}
	if string(body) != "tdx-body" {
		t.Fatalf("body = %q, want tdx-body", string(body))
	}
}
