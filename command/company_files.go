package command

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

type CompanyInfoCategoryCommand struct {
	Market model.Market
	Code   string
}

func NewCompanyInfoCategoryCommand(market model.Market, code string) CompanyInfoCategoryCommand {
	return CompanyInfoCategoryCommand{Market: market, Code: code}
}

func (c CompanyInfoCategoryCommand) Operation() string { return "company_info_category" }

func (c CompanyInfoCategoryCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	req := mustHex("0c0f109b00010e000e00cf02")
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint32(req, 0)
	return req, nil
}

func (c CompanyInfoCategoryCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("company_info_category response truncated: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	out := make([]model.CompanyInfoCategory, 0, count)
	for i := 0; i < count; i++ {
		if pos+152 > len(body) {
			return nil, fmt.Errorf("company_info_category record %d truncated", i)
		}
		raw := append([]byte(nil), body[pos:pos+152]...)
		out = append(out, model.CompanyInfoCategory{
			Name:     decodeTDXString(raw[0:64]),
			Filename: decodeTDXString(raw[64:144]),
			Start:    int(binary.LittleEndian.Uint32(raw[144:148])),
			Length:   int(binary.LittleEndian.Uint32(raw[148:152])),
			Raw:      raw,
		})
		pos += 152
	}
	return out, nil
}

type CompanyInfoContentCommand struct {
	Market   model.Market
	Code     string
	Filename string
	Offset   int
	Length   int
}

func NewCompanyInfoContentCommand(market model.Market, code string, filename string, offset int, length int) CompanyInfoContentCommand {
	return CompanyInfoContentCommand{Market: market, Code: code, Filename: filename, Offset: offset, Length: length}
}

func (c CompanyInfoContentCommand) Operation() string { return "company_info_content" }

func (c CompanyInfoContentCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	filename := [80]byte{}
	copy(filename[:], []byte(c.Filename))
	req := mustHex("0c07109c000168006800d002")
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint16(req, 0)
	req = append(req, filename[:]...)
	req = binary.LittleEndian.AppendUint32(req, uint32(c.Offset))
	req = binary.LittleEndian.AppendUint32(req, uint32(c.Length))
	req = binary.LittleEndian.AppendUint32(req, 0)
	return req, nil
}

func (c CompanyInfoContentCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 12 {
		return nil, fmt.Errorf("company_info_content response truncated: %d", len(body))
	}
	length := int(binary.LittleEndian.Uint16(body[10:12]))
	if 12+length > len(body) {
		return nil, fmt.Errorf("company_info_content body truncated: length=%d actual=%d", length, len(body)-12)
	}
	return append([]byte(nil), body[12:12+length]...), nil
}

type BlockInfoMetaCommand struct {
	Filename string
}

func NewBlockInfoMetaCommand(filename string) BlockInfoMetaCommand {
	return BlockInfoMetaCommand{Filename: filename}
}

func (c BlockInfoMetaCommand) Operation() string { return "block_info_meta" }

func (c BlockInfoMetaCommand) BuildRequest() ([]byte, error) {
	filename := [40]byte{}
	copy(filename[:], []byte(c.Filename))
	req := mustHex("0c39186900012a002a00c502")
	req = append(req, filename[:]...)
	return req, nil
}

func (c BlockInfoMetaCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 38 {
		return nil, fmt.Errorf("block_info_meta response truncated: %d", len(body))
	}
	return model.FileMeta{
		Filename: c.Filename,
		Size:     int(binary.LittleEndian.Uint32(body[0:4])),
		Hash:     strings.TrimRight(string(body[5:37]), "\x00 "),
	}, nil
}

type FileChunkCommand struct {
	OperationName string
	Filename      string
	Start         int
	Length        int
}

func NewBlockInfoCommand(filename string, start int, length int) FileChunkCommand {
	return FileChunkCommand{OperationName: "block_info", Filename: filename, Start: start, Length: length}
}

func NewReportFileCommand(filename string, start int, length int) FileChunkCommand {
	return FileChunkCommand{OperationName: "report_file", Filename: filename, Start: start, Length: length}
}

func (c FileChunkCommand) Operation() string { return c.OperationName }

func (c FileChunkCommand) BuildRequest() ([]byte, error) {
	filename := [100]byte{}
	copy(filename[:], []byte(c.Filename))
	req := mustHex("0c37186a00016e006e00b906")
	req = binary.LittleEndian.AppendUint32(req, uint32(c.Start))
	req = binary.LittleEndian.AppendUint32(req, uint32(c.Length))
	req = append(req, filename[:]...)
	return req, nil
}

func (c FileChunkCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 4 {
		return []byte{}, nil
	}
	return append([]byte(nil), body[4:]...), nil
}

func ParseBlockData(data []byte, filename string) []model.Board {
	if len(data) < 386 {
		return nil
	}
	pos := 384
	count := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
	pos += 2
	category := 0
	switch {
	case strings.Contains(filename, "gn"):
		category = 2
	case strings.Contains(filename, "fg"):
		category = 3
	case strings.Contains(filename, "zs"):
		category = 0
	}
	out := make([]model.Board, 0, count)
	for i := 0; i < count; i++ {
		if pos+2813 > len(data) {
			break
		}
		raw := append([]byte(nil), data[pos:pos+2813]...)
		name := decodeTDXString(raw[0:9])
		stockCount := binary.LittleEndian.Uint16(raw[9:11])
		boardType := int(binary.LittleEndian.Uint16(raw[11:13]))
		if boardType != 0 {
			category = boardType
		}
		codes := make([]string, 0, stockCount)
		actual := int(stockCount)
		if actual > 400 {
			actual = 400
		}
		for j := 0; j < actual; j++ {
			start := 13 + j*7
			code := strings.TrimRight(string(raw[start:start+7]), "\x00 ")
			if code != "" {
				codes = append(codes, code)
			}
		}
		out = append(out, model.Board{Name: name, Category: category, Count: stockCount, Codes: codes, Raw: raw})
		pos += 2813
	}
	return out
}

func decodeTDXString(raw []byte) string {
	if idx := indexByte(raw, 0); idx >= 0 {
		raw = raw[:idx]
	}
	return codec.DecodeGBKBestEffort(raw)
}

func indexByte(raw []byte, b byte) int {
	for i, v := range raw {
		if v == b {
			return i
		}
	}
	return -1
}
