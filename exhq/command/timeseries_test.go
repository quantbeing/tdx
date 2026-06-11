package command_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

func TestMinuteTimeDataCommandBuildRequest(t *testing.T) {
	req, err := command.NewMinuteTimeDataCommand(model.MarketID(47), "IFL0").BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("0107080001010c000c000b24")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
	if req[len(wantPrefix)] != byte(model.MarketID(47)) {
		t.Fatalf("market not encoded: %x", req)
	}
}

func TestParseMinuteTimeDataPreservesRaw(t *testing.T) {
	body := makeMinuteBody(model.MarketID(47), "IFL0", 0)
	got, err := command.ParseMinuteTimeData(body)
	if err != nil {
		t.Fatalf("ParseMinuteTimeData: %v", err)
	}
	assertMinuteRow(t, got, 0)
}

func TestParseHistoryMinuteTimeDataPreservesRawAndUnknown(t *testing.T) {
	body := makeMinuteBody(model.MarketID(47), "IFL0", 8)
	copy(body[10:18], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	got, err := command.ParseHistoryMinuteTimeData(body, 20260611)
	if err != nil {
		t.Fatalf("ParseHistoryMinuteTimeData: %v", err)
	}
	assertMinuteRow(t, got, 20260611)
	if !bytes.Equal(got[0].Unknown, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("unknown = %x", got[0].Unknown)
	}
}

func TestTransactionDataCommandBuildRequest(t *testing.T) {
	req, err := command.NewTransactionDataCommand(model.MarketID(47), "IFL0", 10, 100).BuildRequest()
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	wantPrefix, _ := hex.DecodeString("01010800030112001200fc23")
	if !bytes.Equal(req[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("request prefix = %x want %x", req[:len(wantPrefix)], wantPrefix)
	}
}

func TestParseTransactionDataPreservesRawAndUnknown(t *testing.T) {
	body := makeTransactionBody(model.MarketID(47), "IFL0")
	got, err := command.ParseTransactionData(body, 0)
	if err != nil {
		t.Fatalf("ParseTransactionData: %v", err)
	}
	assertTransactionRow(t, got, 0)
	if !bytes.Equal(got[0].Unknown, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
		t.Fatalf("unknown = %x", got[0].Unknown)
	}
}

func TestParseHistoryTransactionDataPreservesRawAndUnknown(t *testing.T) {
	body := makeTransactionBody(model.MarketID(47), "IFL0")
	got, err := command.ParseHistoryTransactionData(body, 20260611)
	if err != nil {
		t.Fatalf("ParseHistoryTransactionData: %v", err)
	}
	assertTransactionRow(t, got, 20260611)
	if !bytes.Equal(got[0].Unknown, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
		t.Fatalf("unknown = %x", got[0].Unknown)
	}
}

func makeMinuteBody(market model.MarketID, code string, unknownLen int) []byte {
	headerLen := 12
	if unknownLen > 0 {
		headerLen = 20
	}
	body := make([]byte, headerLen+18)
	body[0] = byte(market)
	copy(body[1:10], []byte(code))
	binary.LittleEndian.PutUint16(body[headerLen-2:headerLen], 1)
	row := body[headerLen:]
	binary.LittleEndian.PutUint16(row[0:2], 9*60+30)
	writeFloat32(row[2:6], 3706.5)
	writeFloat32(row[6:10], 3706.25)
	binary.LittleEndian.PutUint32(row[10:14], 27)
	binary.LittleEndian.PutUint32(row[14:18], 13336)
	return body
}

func assertMinuteRow(t *testing.T, got []model.MinuteTime, date int) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	row := got[0]
	if row.Market != model.MarketID(47) || row.Code != "IFL0" || row.Date != date || row.Hour != 9 || row.Minute != 30 || row.Price != 3706.5 || row.AvgPrice != 3706.25 || row.Volume != 27 || row.OpenInterest != 13336 {
		t.Fatalf("minute = %+v", row)
	}
	if !bytes.Equal(row.Raw, []byte{0x3a, 0x02, 0x00, 0xa8, 0x67, 0x45, 0x00, 0xa4, 0x67, 0x45, 0x1b, 0, 0, 0, 0x18, 0x34, 0, 0}) {
		t.Fatalf("raw = %x", row.Raw)
	}
}

func makeTransactionBody(market model.MarketID, code string) []byte {
	body := make([]byte, 16+16)
	body[0] = byte(market)
	copy(body[1:10], []byte(code))
	copy(body[10:14], []byte{0xaa, 0xbb, 0xcc, 0xdd})
	binary.LittleEndian.PutUint16(body[14:16], 1)
	row := body[16:]
	binary.LittleEndian.PutUint16(row[0:2], 9*60+31)
	binary.LittleEndian.PutUint32(row[2:6], 370650)
	binary.LittleEndian.PutUint32(row[6:10], 12)
	binary.LittleEndian.PutUint32(row[10:14], 0xfffffffd)
	binary.LittleEndian.PutUint16(row[14:16], 10007)
	return body
}

func assertTransactionRow(t *testing.T, got []model.Transaction, date int) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	row := got[0]
	if row.Market != model.MarketID(47) || row.Code != "IFL0" || row.Date != date || row.Hour != 9 || row.Minute != 31 || row.Second != 7 || row.Price != 370650 || row.Volume != 12 || row.PositionChange != -3 || row.Nature != 10007 || row.NatureMark != 1 || row.NatureValue != 7 {
		t.Fatalf("transaction = %+v", row)
	}
	if !bytes.Equal(row.Raw, []byte{0x3b, 0x02, 0xda, 0xa7, 0x05, 0x00, 0x0c, 0, 0, 0, 0xfd, 0xff, 0xff, 0xff, 0x17, 0x27}) {
		t.Fatalf("raw = %x", row.Raw)
	}
}
