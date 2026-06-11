package main

import (
	"os"
	"strings"
	"testing"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

func TestWriteCaptureFixtureUsesDefaultPath(t *testing.T) {
	dir := t.TempDir()
	capture := tdx.CapturedResponse{
		Operation: "security_count",
		Server:    model.Server{Name: "fake", Host: "127.0.0.1", Port: 7709},
		Header:    frame.Header{ZipSize: 1, UnzipSize: 1},
		RawBody:   []byte{1},
		Body:      []byte{1},
		Parsed:    uint16(1),
	}

	summary, err := writeCaptureFixture(dir, capture)
	if err != nil {
		t.Fatalf("writeCaptureFixture: %v", err)
	}
	if !strings.Contains(summary.Path, "security_count") {
		t.Fatalf("path = %s", summary.Path)
	}
	if _, err := os.Stat(summary.Path); err != nil {
		t.Fatalf("fixture not written: %v", err)
	}
}

func TestCommandForOptionsBuildsMultiMarketQuote(t *testing.T) {
	cmd, err := commandForOptions(probeOptions{
		Op:      "quote",
		Symbols: "sh:600519,sz:000001",
	})
	if err != nil {
		t.Fatalf("commandForOptions: %v", err)
	}
	quoteCmd, ok := cmd.(command.SecurityQuotesCommand)
	if !ok {
		t.Fatalf("cmd type = %T", cmd)
	}
	if len(quoteCmd.Symbols) != 2 ||
		quoteCmd.Symbols[0] != (model.Symbol{Market: model.MarketSH, Code: "600519"}) ||
		quoteCmd.Symbols[1] != (model.Symbol{Market: model.MarketSZ, Code: "000001"}) {
		t.Fatalf("symbols = %+v", quoteCmd.Symbols)
	}
}

func TestCommandForOptionsUsesMarketCodeAndCount(t *testing.T) {
	cmd, err := commandForOptions(probeOptions{
		Op:     "transaction",
		Market: "sz",
		Code:   "000001",
		Start:  10,
		Count:  20,
	})
	if err != nil {
		t.Fatalf("commandForOptions: %v", err)
	}
	txCmd, ok := cmd.(command.TransactionDataCommand)
	if !ok {
		t.Fatalf("cmd type = %T", cmd)
	}
	if txCmd.Market != model.MarketSZ || txCmd.Code != "000001" || txCmd.Start != 10 || txCmd.Count != 20 {
		t.Fatalf("transaction command = %+v", txCmd)
	}
}

func TestParseSymbolsRejectsInvalidToken(t *testing.T) {
	_, err := parseSymbols("sh600519")
	if err == nil || !strings.Contains(err.Error(), "market:code") {
		t.Fatalf("err = %v", err)
	}
}
