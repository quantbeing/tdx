// Package frame parses TDX response frames.
package frame

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

const HeaderSize = 16

// Header is the fixed 16-byte response header used by the TDX TCP protocol.
type Header struct {
	Unknown0  uint32
	Unknown1  uint32
	Unknown2  uint32
	ZipSize   uint16
	UnzipSize uint16
}

func ParseHeader(buf []byte) (Header, error) {
	if len(buf) < HeaderSize {
		return Header{}, fmt.Errorf("tdx frame header truncated: got %d bytes", len(buf))
	}
	return Header{
		Unknown0:  binary.LittleEndian.Uint32(buf[0:4]),
		Unknown1:  binary.LittleEndian.Uint32(buf[4:8]),
		Unknown2:  binary.LittleEndian.Uint32(buf[8:12]),
		ZipSize:   binary.LittleEndian.Uint16(buf[12:14]),
		UnzipSize: binary.LittleEndian.Uint16(buf[14:16]),
	}, nil
}

func DecodeBody(header Header, raw []byte) ([]byte, error) {
	if len(raw) != int(header.ZipSize) {
		return nil, fmt.Errorf("tdx frame body length mismatch: header=%d actual=%d", header.ZipSize, len(raw))
	}
	if header.ZipSize == header.UnzipSize {
		return raw, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("tdx frame zlib reader: %w", err)
	}
	defer zr.Close()
	body, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("tdx frame zlib read: %w", err)
	}
	if len(body) != int(header.UnzipSize) {
		return nil, fmt.Errorf("tdx frame unzip length mismatch: header=%d actual=%d", header.UnzipSize, len(body))
	}
	return body, nil
}
