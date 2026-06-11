package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/exhq/model"
)

const marketRecordSize = 64

type MarketsCommand struct{}

func NewMarketsCommand() MarketsCommand {
	return MarketsCommand{}
}

func (c MarketsCommand) Operation() string { return "exhq_markets" }

func (c MarketsCommand) BuildRequest() ([]byte, error) {
	return mustHex("01024869000102000200f423"), nil
}

func (c MarketsCommand) ParseResponse(body []byte) (any, error) {
	return ParseMarkets(body)
}

func ParseMarkets(body []byte) ([]model.Market, error) {
	if err := requireLen("exhq_markets", body, 2); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint16(body[0:2]))
	pos := 2
	out := make([]model.Market, 0, count)
	for i := 0; i < count; i++ {
		if pos+marketRecordSize > len(body) {
			return nil, fmt.Errorf("exhq_markets record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+marketRecordSize]...)
		pos += marketRecordSize
		category := raw[0]
		marketID := model.MarketID(raw[33])
		if category == 0 && marketID == 0 {
			continue
		}
		out = append(out, model.Market{
			MarketID:  marketID,
			Category:  category,
			Name:      decodeText(raw[1:33]),
			ShortName: decodeText(raw[34:36]),
			Unknown:   append([]byte(nil), raw[62:64]...),
			Raw:       raw,
		})
	}
	return out, nil
}
