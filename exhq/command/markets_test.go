package command_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

func TestMarketsCommandBuildRequest(t *testing.T) {
	req, err := command.NewMarketsCommand().BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	want, _ := hex.DecodeString("01024869000102000200f423")
	if !bytes.Equal(req, want) {
		t.Fatalf("request = %x want %x", req, want)
	}
}

func TestParseMarketsPreservesRawAndUnknown(t *testing.T) {
	body := make([]byte, 2+64)
	binary.LittleEndian.PutUint16(body[0:2], 1)
	row := body[2:]
	row[0] = 1
	copy(row[1:33], []byte("Futures\x00"))
	row[33] = byte(model.MarketID(47))
	copy(row[34:36], []byte("IF"))
	row[62] = 0xaa
	row[63] = 0xbb

	got, err := command.ParseMarkets(body)
	if err != nil {
		t.Fatalf("ParseMarkets: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	if got[0].MarketID != model.MarketID(47) || got[0].Category != 1 || got[0].Name != "Futures" || got[0].ShortName != "IF" {
		t.Fatalf("market = %+v", got[0])
	}
	if !bytes.Equal(got[0].Unknown, []byte{0xaa, 0xbb}) {
		t.Fatalf("unknown = %x", got[0].Unknown)
	}
	if !bytes.Equal(got[0].Raw, row) {
		t.Fatalf("raw = %x want %x", got[0].Raw, row)
	}
}
