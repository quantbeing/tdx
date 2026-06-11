package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/exhq/model"
)

const instrumentRecordSize = 64

type InstrumentCountCommand struct{}

func NewInstrumentCountCommand() InstrumentCountCommand {
	return InstrumentCountCommand{}
}

func (c InstrumentCountCommand) Operation() string { return "exhq_instrument_count" }

func (c InstrumentCountCommand) BuildRequest() ([]byte, error) {
	return mustHex("01034866000102000200f023"), nil
}

func (c InstrumentCountCommand) ParseResponse(body []byte) (any, error) {
	return ParseInstrumentCount(body)
}

func ParseInstrumentCount(body []byte) (int, error) {
	if err := requireLen("exhq_instrument_count", body, 23); err != nil {
		return 0, err
	}
	return int(binary.LittleEndian.Uint32(body[19:23])), nil
}

type InstrumentInfoCommand struct {
	Start int
	Count int
}

func NewInstrumentInfoCommand(start, count int) InstrumentInfoCommand {
	return InstrumentInfoCommand{Start: start, Count: count}
}

func (c InstrumentInfoCommand) Operation() string { return "exhq_instrument_info" }

func (c InstrumentInfoCommand) BuildRequest() ([]byte, error) {
	req := mustHex("01044867000108000800f523")
	req = putUint32(req, c.Start)
	req = putUint16(req, c.Count)
	return req, nil
}

func (c InstrumentInfoCommand) ParseResponse(body []byte) (any, error) {
	return ParseInstrumentInfo(body)
}

func ParseInstrumentInfo(body []byte) ([]model.Instrument, error) {
	if err := requireLen("exhq_instrument_info", body, 6); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint16(body[4:6]))
	pos := 6
	out := make([]model.Instrument, 0, count)
	for i := 0; i < count; i++ {
		if pos+instrumentRecordSize > len(body) {
			return nil, fmt.Errorf("exhq_instrument_info record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+instrumentRecordSize]...)
		pos += instrumentRecordSize
		var unknown1 [3]byte
		copy(unknown1[:], raw[2:5])
		out = append(out, model.Instrument{
			Category: raw[0],
			Market:   model.MarketID(raw[1]),
			Code:     trimCode(raw[5:14]),
			Name:     decodeText(raw[14:31]),
			Desc:     decodeText(raw[31:40]),
			Unknown1: unknown1,
			Unknown:  append([]byte(nil), raw[40:64]...),
			Raw:      raw,
		})
	}
	return out, nil
}
