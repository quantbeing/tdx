package command

import (
	"encoding/binary"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestHistoryFundFlowParserKeepsDateAmountsAndRaw(t *testing.T) {
	cmd := NewHistoryFundFlowCommand(model.MarketSH, "600519", 0, 1)
	req, err := cmd.BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if len(req) != 38 {
		t.Fatalf("request len = %d, want 38", len(req))
	}
	if binary.LittleEndian.Uint16(req[20:22]) != 22 {
		t.Fatalf("category = %d, want 22", binary.LittleEndian.Uint16(req[20:22]))
	}

	body := make([]byte, 11+36)
	binary.LittleEndian.PutUint16(body[9:11], 1)
	binary.LittleEndian.PutUint32(body[11:15], 20260609)

	reply, err := cmd.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	rows := reply.([]model.HistoricalFundFlow)
	if len(rows) != 1 {
		t.Fatalf("len = %d", len(rows))
	}
	if rows[0].Year != 2026 || rows[0].Month != 6 || rows[0].Day != 9 {
		t.Fatalf("date = %+v", rows[0])
	}
	if rows[0].SuperIn != 0 || rows[0].SmallOut != 0 || len(rows[0].Raw) != 36 {
		t.Fatalf("row = %+v raw=%x", rows[0], rows[0].Raw)
	}
}
