package command

import "encoding/hex"

var SetupCommands = [][]byte{
	mustHex("0c0218930001030003000d0001"),
	mustHex("0c0218940001030003000d0002"),
	mustHex("0c031899000120002000db0fd5d0c9ccd6a4a8af0000008fc22540130000d500c9ccbdf0d7ea00000002"),
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
