package command

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestFinanceInfoParserScalesFieldsAndPreservesRaw(t *testing.T) {
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = append(body, byte(model.MarketSH))
	body = append(body, []byte("600519")...)
	body = appendFloat32(body, 123.5)
	body = binary.LittleEndian.AppendUint16(body, 11)
	body = binary.LittleEndian.AppendUint16(body, 22)
	body = binary.LittleEndian.AppendUint32(body, 20260609)
	body = binary.LittleEndian.AppendUint32(body, 20010827)
	for i := 0; i < 30; i++ {
		body = appendFloat32(body, float32(i+1))
	}

	reply, err := NewFinanceInfoCommand(model.MarketSH, "600519").ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	info := reply.(model.FinanceInfo)
	if info.Market != model.MarketSH || info.Code != "600519" || info.Province != 11 || info.Industry != 22 {
		t.Fatalf("identity/meta = %+v", info)
	}
	if info.LiutongGuben != 1235000 || info.ZongGuben != 10000 || info.UpdatedDate != 20260609 || info.IPODate != 20010827 {
		t.Fatalf("scaled fields = %+v", info)
	}
	if len(info.Raw) == 0 {
		t.Fatal("Raw is empty")
	}
}

func TestXdxrParserReadsRecordHeaderAtCurrentPositionAndDecodesShareCounts(t *testing.T) {
	body := make([]byte, 9)
	body = binary.LittleEndian.AppendUint16(body, 2)
	body = appendXdxrShareRecord(body, model.MarketSH, "600519", 20260609, 5)
	body = appendXdxrShareRecord(body, model.MarketSZ, "000001", 20260610, 6)

	reply, err := NewXdxrInfoCommand(model.MarketSH, "600519").ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	rows := reply.([]model.XdxrRecord)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Code != "600519" || rows[1].Code != "000001" || rows[1].Market != model.MarketSZ {
		t.Fatalf("record header position bug still present: %+v", rows)
	}
	if rows[0].PanqianLiutong == nil || math.Abs(*rows[0].PanqianLiutong-5767489) > 1 {
		t.Fatalf("share count = %v, want volume-decoded 5767489", rows[0].PanqianLiutong)
	}
	if len(rows[0].Raw) == 0 || len(rows[1].Raw) == 0 {
		t.Fatal("Raw is empty")
	}
}

func appendFloat32(out []byte, v float32) []byte {
	var bits [4]byte
	binary.LittleEndian.PutUint32(bits[:], math.Float32bits(v))
	return append(out, bits[:]...)
}

func appendXdxrShareRecord(out []byte, market model.Market, code string, date uint32, category byte) []byte {
	out = append(out, byte(market))
	codeBytes := [6]byte{}
	copy(codeBytes[:], []byte(code))
	out = append(out, codeBytes[:]...)
	out = append(out, 0)
	out = binary.LittleEndian.AppendUint32(out, date)
	out = append(out, category)
	for i := 0; i < 4; i++ {
		out = binary.LittleEndian.AppendUint32(out, 0x4ab00282)
	}
	return out
}
