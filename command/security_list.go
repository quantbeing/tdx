package command

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

const securityListRecordSize = 29

type SecurityListCommand struct {
	Market model.Market
	Start  int
}

func NewSecurityListCommand(market model.Market, start int) SecurityListCommand {
	return SecurityListCommand{Market: market, Start: start}
}

func (c SecurityListCommand) Operation() string { return "security_list" }

func (c SecurityListCommand) BuildRequest() ([]byte, error) {
	req := mustHex("0c0118640101060006005004")
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Start))
	req = binary.LittleEndian.AppendUint16(req, 0)
	return req, nil
}

func (c SecurityListCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("security_list header truncated: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[0:2]))
	pos := 2
	out := make([]model.Security, 0, count)
	for i := 0; i < count; i++ {
		if pos+securityListRecordSize > len(body) {
			return nil, fmt.Errorf("security_list record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+securityListRecordSize]...)
		var unknown1, unknown2 [4]byte
		copy(unknown1[:], raw[16:20])
		copy(unknown2[:], raw[25:29])
		out = append(out, model.Security{
			Market:       c.Market,
			Code:         strings.TrimRight(string(raw[0:6]), "\x00 "),
			VolUnit:      binary.LittleEndian.Uint16(raw[6:8]),
			Name:         codec.DecodeGBKBestEffort(raw[8:16]),
			Unknown1:     unknown1,
			DecimalPoint: raw[20],
			PreClose:     codec.DecodeVolume(binary.LittleEndian.Uint32(raw[21:25])),
			Unknown2:     unknown2,
			Raw:          raw,
		})
		pos += securityListRecordSize
	}
	return out, nil
}
