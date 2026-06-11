package command_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

func TestInstrumentBarsCommandBuildRequest(t *testing.T) {
	req, err := command.NewInstrumentBarsCommand(model.KlineDay, model.MarketID(47), "IFL0", 10, 240).BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("0101086a010116001600ff23")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
	if req[len(wantPrefix)] != byte(model.MarketID(47)) || string(bytes.TrimRight(req[len(wantPrefix)+1:len(wantPrefix)+10], "\x00")) != "IFL0" {
		t.Fatalf("market/code not encoded: %x", req)
	}
}

func TestParseInstrumentBarsPreservesRaw(t *testing.T) {
	body := make([]byte, 20+32)
	copy(body[:18], []byte("bars fixture head"))
	binary.LittleEndian.PutUint16(body[18:20], 1)
	row := body[20:]
	binary.LittleEndian.PutUint32(row[0:4], 20260611)
	writeFloat32(row[4:8], 10.5)
	writeFloat32(row[8:12], 12.5)
	writeFloat32(row[12:16], 9.5)
	writeFloat32(row[16:20], 11.5)
	binary.LittleEndian.PutUint32(row[20:24], 300)
	binary.LittleEndian.PutUint32(row[24:28], 400)
	writeFloat32(row[28:32], 11.25)

	got, err := command.ParseInstrumentBars(body, model.KlineDay)
	if err != nil {
		t.Fatalf("ParseInstrumentBars: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	bar := got[0]
	if bar.Year != 2026 || bar.Month != 6 || bar.Day != 11 || bar.Open != 10.5 || bar.Close != 11.5 || bar.Position != 300 || bar.Trade != 400 || bar.Price != 11.25 {
		t.Fatalf("bar = %+v", bar)
	}
	if len(bar.Unknown) != 0 {
		t.Fatalf("unknown = %x", bar.Unknown)
	}
	if !bytes.Equal(bar.Raw, row) {
		t.Fatalf("raw = %x want %x", bar.Raw, row)
	}
}

func TestParseHistoryInstrumentBarsRangePreservesRaw(t *testing.T) {
	body := make([]byte, 14+32)
	copy(body[:12], []byte("history head"))
	binary.LittleEndian.PutUint16(body[12:14], 1)
	row := body[14:]
	binary.LittleEndian.PutUint16(row[0:2], zipExDate(2026, 6, 11))
	binary.LittleEndian.PutUint16(row[2:4], 9*60+31)
	writeFloat32(row[4:8], 10.5)
	writeFloat32(row[8:12], 12.5)
	writeFloat32(row[12:16], 9.5)
	writeFloat32(row[16:20], 11.5)
	binary.LittleEndian.PutUint32(row[20:24], 300)
	binary.LittleEndian.PutUint32(row[24:28], 400)
	writeFloat32(row[28:32], 11.25)

	got, err := command.ParseHistoryInstrumentBarsRange(body)
	if err != nil {
		t.Fatalf("ParseHistoryInstrumentBarsRange: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	bar := got[0]
	if bar.Year != 2026 || bar.Month != 6 || bar.Day != 11 || bar.Hour != 9 || bar.Minute != 31 || bar.SettlementPrice != 11.25 {
		t.Fatalf("bar = %+v", bar)
	}
	if len(bar.Unknown) != 0 {
		t.Fatalf("unknown = %x", bar.Unknown)
	}
	if !bytes.Equal(bar.Raw, row) {
		t.Fatalf("raw = %x want %x", bar.Raw, row)
	}
}

func zipExDate(year, month, day int) uint16 {
	return uint16((year-2004)<<11 + month*100 + day)
}
