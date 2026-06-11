package command_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

func TestInstrumentCountCommandBuildRequest(t *testing.T) {
	req, err := command.NewInstrumentCountCommand().BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	want, _ := hex.DecodeString("01034866000102000200f023")
	if !bytes.Equal(req, want) {
		t.Fatalf("request = %x want %x", req, want)
	}
}

func TestParseInstrumentCount(t *testing.T) {
	body := make([]byte, 23)
	copy(body[:], []byte("TDX_EXHQ_COUNT_DATA"))
	binary.LittleEndian.PutUint32(body[19:23], 321)

	got, err := command.ParseInstrumentCount(body)
	if err != nil {
		t.Fatalf("ParseInstrumentCount: %v", err)
	}
	if got != 321 {
		t.Fatalf("count = %d want 321", got)
	}
}

func TestInstrumentInfoCommandBuildRequest(t *testing.T) {
	req, err := command.NewInstrumentInfoCommand(200, 100).BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("01044867000108000800f523")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
	if binary.LittleEndian.Uint32(req[len(wantPrefix):len(wantPrefix)+4]) != 200 {
		t.Fatalf("start not encoded in request: %x", req)
	}
	if binary.LittleEndian.Uint16(req[len(wantPrefix)+4:]) != 100 {
		t.Fatalf("count not encoded in request: %x", req)
	}
}

func TestParseInstrumentInfoPreservesRawAndUnknown(t *testing.T) {
	body := make([]byte, 6+64)
	binary.LittleEndian.PutUint32(body[0:4], 200)
	binary.LittleEndian.PutUint16(body[4:6], 1)
	row := body[6:]
	row[0] = 3
	row[1] = byte(model.MarketID(47))
	copy(row[2:5], []byte{0x10, 0x20, 0x30})
	copy(row[5:14], []byte("IFL0\x00"))
	copy(row[14:31], []byte("Index Future\x00"))
	copy(row[31:40], []byte("main\x00"))
	for i := 40; i < 64; i++ {
		row[i] = byte(i)
	}

	got, err := command.ParseInstrumentInfo(body)
	if err != nil {
		t.Fatalf("ParseInstrumentInfo: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	inst := got[0]
	if inst.Category != 3 || inst.Market != model.MarketID(47) || inst.Code != "IFL0" || inst.Name != "Index Future" || inst.Desc != "main" {
		t.Fatalf("instrument = %+v", inst)
	}
	if inst.Unknown1 != [3]byte{0x10, 0x20, 0x30} {
		t.Fatalf("unknown1 = %x", inst.Unknown1)
	}
	if !bytes.Equal(inst.Unknown, row[40:64]) {
		t.Fatalf("unknown tail = %x", inst.Unknown)
	}
	if !bytes.Equal(inst.Raw, row) {
		t.Fatalf("raw = %x want %x", inst.Raw, row)
	}
}
