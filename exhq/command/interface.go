package command

import (
	"encoding/hex"
	"fmt"
)

type Command interface {
	BuildRequest() ([]byte, error)
	ParseResponse(body []byte) (any, error)
	Operation() string
}

type UnsupportedCommand struct {
	Name string
}

func (c UnsupportedCommand) BuildRequest() ([]byte, error) {
	return nil, ErrUnsupported{Operation: c.Name}
}

func (c UnsupportedCommand) ParseResponse([]byte) (any, error) {
	return nil, ErrUnsupported{Operation: c.Name}
}

func (c UnsupportedCommand) Operation() string {
	return c.Name
}

type ErrUnsupported struct {
	Operation string
}

func (e ErrUnsupported) Error() string {
	return "tdx exhq command unsupported: " + e.Operation
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func requireLen(op string, body []byte, need int) error {
	if len(body) < need {
		return fmt.Errorf("%s response truncated: %d < %d", op, len(body), need)
	}
	return nil
}
