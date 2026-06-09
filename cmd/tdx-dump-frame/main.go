package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/quantbeing/tdx/frame"
)

func main() {
	var raw string
	flag.StringVar(&raw, "hex", "", "hex-encoded 16-byte header followed by body")
	flag.Parse()
	raw = strings.ReplaceAll(raw, " ", "")
	if raw == "" {
		fmt.Fprintln(os.Stderr, "missing -hex")
		os.Exit(2)
	}
	data, err := hex.DecodeString(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	header, err := frame.ParseHeader(data[:min(len(data), frame.HeaderSize)])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	body, err := frame.DecodeBody(header, data[frame.HeaderSize:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"header": header, "body_hex": hex.EncodeToString(body)})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
