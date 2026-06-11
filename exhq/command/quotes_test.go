package command_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

func TestInstrumentQuoteCommandBuildRequest(t *testing.T) {
	req, err := command.NewInstrumentQuoteCommand(model.MarketID(47), "IFL0").BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("0101080202010c000c00fa23")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
	if req[len(wantPrefix)] != byte(model.MarketID(47)) {
		t.Fatalf("market not encoded in request: %x", req)
	}
	if string(bytes.TrimRight(req[len(wantPrefix)+1:], "\x00")) != "IFL0" {
		t.Fatalf("code not encoded in request: %x", req)
	}
}

func TestParseInstrumentQuotePreservesRawAndUnknown(t *testing.T) {
	body := makeSingleQuoteBody(t)

	got, err := command.ParseInstrumentQuote(body)
	if err != nil {
		t.Fatalf("ParseInstrumentQuote: %v", err)
	}
	if got.Market != model.MarketID(47) || got.Code != "IFL0" || got.Price != 14.5 || got.PreClose != 10.5 {
		t.Fatalf("quote = %+v", got)
	}
	if got.OpenInterest != 666 || got.Bid[0].Price != 15.5 || got.Bid[0].Volume != 1001 || got.Ask[0].Price != 20.5 {
		t.Fatalf("levels/open interest = %+v", got)
	}
	if got.Unknown1 != 999 || got.Unknown2 != 888 || got.Unknown3 != 777 {
		t.Fatalf("unknown integers = %d %d %d", got.Unknown1, got.Unknown2, got.Unknown3)
	}
	if !bytes.Equal(got.Unknown, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
		t.Fatalf("unknown header = %x", got.Unknown)
	}
	if !bytes.Equal(got.Raw, body) {
		t.Fatalf("raw = %x want %x", got.Raw, body)
	}
}

func TestInstrumentQuoteListCommandBuildRequest(t *testing.T) {
	req, err := command.NewInstrumentQuoteListCommand(model.MarketID(29), 3, 10, 2).BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("01c1060b00020b000b000024")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
	tail := req[len(wantPrefix):]
	if tail[0] != byte(model.MarketID(29)) || binary.LittleEndian.Uint16(tail[3:5]) != 10 || binary.LittleEndian.Uint16(tail[5:7]) != 2 {
		t.Fatalf("request tail = %x", tail)
	}
}

func TestParseInstrumentQuoteListPreservesRawAndUnknown(t *testing.T) {
	body := make([]byte, 2+300)
	binary.LittleEndian.PutUint16(body[0:2], 1)
	row := body[2:]
	row[0] = byte(model.MarketID(29))
	copy(row[1:10], []byte("IFL0\x00"))
	writeFloat32(row[14:18], 10.5)
	writeFloat32(row[18:22], 11.5)
	writeFloat32(row[22:26], 12.5)
	writeFloat32(row[26:30], 9.5)
	writeFloat32(row[30:34], 14.5)
	for i := 150; i < len(row); i++ {
		row[i] = byte(i)
	}

	got, err := command.ParseInstrumentQuoteList(body, 3)
	if err != nil {
		t.Fatalf("ParseInstrumentQuoteList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	if got[0].Market != model.MarketID(29) || got[0].Code != "IFL0" || got[0].PreClose != 10.5 || got[0].Price != 14.5 {
		t.Fatalf("quote = %+v", got[0])
	}
	if !bytes.Equal(got[0].Unknown, row[150:300]) {
		t.Fatalf("unknown tail = %x", got[0].Unknown)
	}
	if !bytes.Equal(got[0].Raw, row) {
		t.Fatalf("raw = %x want %x", got[0].Raw, row)
	}
}

func makeSingleQuoteBody(t *testing.T) []byte {
	t.Helper()
	body := make([]byte, 0, 150)
	body = append(body, byte(model.MarketID(47)))
	body = appendPadded(body, "IFL0", 9)
	body = append(body, 0xaa, 0xbb, 0xcc, 0xdd)
	for _, v := range []float32{10.5, 11.5, 12.5, 9.5, 14.5} {
		body = appendFloat32(body, v)
	}
	for _, v := range []uint32{111, 999, 222, 333, 888, 444, 555, 777, 666} {
		body = binary.LittleEndian.AppendUint32(body, v)
	}
	for _, v := range []float32{15.5, 15.25, 15, 14.75, 14.5} {
		body = appendFloat32(body, v)
	}
	for _, v := range []uint32{1001, 1002, 1003, 1004, 1005} {
		body = binary.LittleEndian.AppendUint32(body, v)
	}
	for _, v := range []float32{20.5, 20.75, 21, 21.25, 21.5} {
		body = appendFloat32(body, v)
	}
	for _, v := range []uint32{2001, 2002, 2003, 2004, 2005} {
		body = binary.LittleEndian.AppendUint32(body, v)
	}
	if len(body) != 150 {
		t.Fatalf("fixture len = %d want 150", len(body))
	}
	return body
}

func appendPadded(dst []byte, s string, n int) []byte {
	var buf [16]byte
	copy(buf[:], []byte(s))
	return append(dst, buf[:n]...)
}

func appendFloat32(dst []byte, v float32) []byte {
	var buf [4]byte
	writeFloat32(buf[:], v)
	return append(dst, buf[:]...)
}

func writeFloat32(dst []byte, v float32) {
	binary.LittleEndian.PutUint32(dst, math.Float32bits(v))
}
