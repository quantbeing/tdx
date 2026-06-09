package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/model"
)

type SecurityCountCommand struct {
	Market model.Market
}

func NewSecurityCountCommand(market model.Market) SecurityCountCommand {
	return SecurityCountCommand{Market: market}
}

func (c SecurityCountCommand) Operation() string { return "security_count" }

func (c SecurityCountCommand) BuildRequest() ([]byte, error) {
	req := mustHex("0c0c186c0001080008004e04")
	req = append(req, 0, 0, 0x75, 0xc7, 0x33, 0x01)
	binary.LittleEndian.PutUint16(req[len(req)-6:len(req)-4], uint16(c.Market))
	return req, nil
}

func (c SecurityCountCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("security_count response truncated: %d", len(body))
	}
	return binary.LittleEndian.Uint16(body[:2]), nil
}
