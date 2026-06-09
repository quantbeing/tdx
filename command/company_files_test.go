package command

import (
	"encoding/binary"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestCompanyInfoCategoryParserDecodesGBKAndOffsets(t *testing.T) {
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	row := make([]byte, 152)
	copy(row[0:64], []byte{0xd7, 0xee, 0xd0, 0xc2, 0xcc, 0xe1, 0xca, 0xbe}) // 最新提示
	copy(row[64:144], []byte("600519.txt"))
	binary.LittleEndian.PutUint32(row[144:148], 1234)
	binary.LittleEndian.PutUint32(row[148:152], 5678)
	body = append(body, row...)

	reply, err := NewCompanyInfoCategoryCommand(model.MarketSH, "600519").ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	items := reply.([]model.CompanyInfoCategory)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Name != "最新提示" || items[0].Filename != "600519.txt" ||
		items[0].Start != 1234 || items[0].Length != 5678 || len(items[0].Raw) == 0 {
		t.Fatalf("item = %+v", items[0])
	}
}

func TestCompanyInfoContentParserReturnsBodyBytes(t *testing.T) {
	content := []byte{0xd7, 0xee, 0xd0, 0xc2}
	body := make([]byte, 12)
	binary.LittleEndian.PutUint16(body[10:12], uint16(len(content)))
	body = append(body, content...)
	reply, err := NewCompanyInfoContentCommand(model.MarketSH, "600519", "600519.txt", 100, len(content)).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	got := reply.([]byte)
	if string(got) != string(content) {
		t.Fatalf("content = %x, want %x", got, content)
	}
}

func TestBlockMetaAndChunkParsers(t *testing.T) {
	metaBody := make([]byte, 38)
	binary.LittleEndian.PutUint32(metaBody[0:4], 35000)
	copy(metaBody[5:37], []byte("0123456789abcdef0123456789abcdef"))
	meta, err := NewBlockInfoMetaCommand("block_gn.dat").ParseResponse(metaBody)
	if err != nil {
		t.Fatalf("meta ParseResponse: %v", err)
	}
	gotMeta := meta.(model.FileMeta)
	if gotMeta.Size != 35000 || gotMeta.Hash != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("meta = %+v", gotMeta)
	}

	chunk, err := NewBlockInfoCommand("block_gn.dat", 0, 3).ParseResponse([]byte{0, 0, 0, 0, 1, 2, 3})
	if err != nil {
		t.Fatalf("chunk ParseResponse: %v", err)
	}
	if string(chunk.([]byte)) != string([]byte{1, 2, 3}) {
		t.Fatalf("chunk = %v", chunk)
	}
}

func TestParseBlockDatPreservesMembers(t *testing.T) {
	data := make([]byte, 384)
	data = binary.LittleEndian.AppendUint16(data, 1)
	record := make([]byte, 2813)
	copy(record[0:9], []byte{0xc8, 0xcb, 0xb9, 0xa4, 0xd6, 0xc7, 0xc4, 0xdc}) // 人工智能
	binary.LittleEndian.PutUint16(record[9:11], 2)
	binary.LittleEndian.PutUint16(record[11:13], 2)
	copy(record[13:20], []byte("600519"))
	copy(record[20:27], []byte("000001"))
	data = append(data, record...)

	boards := ParseBlockData(data, "block_gn.dat")
	if len(boards) != 1 {
		t.Fatalf("len(boards) = %d, want 1", len(boards))
	}
	if boards[0].Name != "人工智能" || boards[0].Category != 2 || len(boards[0].Codes) != 2 ||
		boards[0].Codes[0] != "600519" || len(boards[0].Raw) == 0 {
		t.Fatalf("board = %+v", boards[0])
	}
}
