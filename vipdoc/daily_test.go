package vipdoc

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestDailyParsesDayFileAndPreservesRaw(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vipdoc", "sh", "lday", "sh600000.day")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	record := dailyRecord(20260611, 1234, 1250, 1200, 1244, 123456.5, 1000, 7)
	if err := os.WriteFile(path, record, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	bars, err := reader.Daily(context.Background(), model.Symbol{Market: model.MarketSH, Code: "600000"})
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}

	if len(bars) != 1 {
		t.Fatalf("len(bars) = %d, want 1", len(bars))
	}
	got := bars[0]
	if got.Market != model.MarketSH || got.Code != "600000" || got.Date != 20260611 {
		t.Fatalf("bar identity = %+v", got)
	}
	assertFloat(t, "Open", got.Open, 12.34)
	assertFloat(t, "High", got.High, 12.50)
	assertFloat(t, "Low", got.Low, 12.00)
	assertFloat(t, "Close", got.Close, 12.44)
	assertFloat(t, "Amount", got.Amount, 123456.5)
	if got.Volume != 1000 || got.Reserved != 7 {
		t.Fatalf("Volume/Reserved = %d/%d, want 1000/7", got.Volume, got.Reserved)
	}
	if got.RawOpen != 1234 || got.RawHigh != 1250 || got.RawLow != 1200 || got.RawClose != 1244 {
		t.Fatalf("raw prices = %d/%d/%d/%d", got.RawOpen, got.RawHigh, got.RawLow, got.RawClose)
	}
	if string(got.Raw) != string(record) {
		t.Fatalf("Raw = %x, want %x", got.Raw, record)
	}
}

func TestDailyTruncatedFileReportsPathOffsetExpectedAndActual(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vipdoc", "sh", "lday", "sh600000.day")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data := append(dailyRecord(20260610, 1000, 1001, 999, 1000, 1, 2, 3), []byte{1, 2, 3}...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = reader.Daily(context.Background(), model.Symbol{Market: model.MarketSH, Code: "600000"})
	if err == nil {
		t.Fatal("Daily succeeded, want truncation error")
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %T = %v, want *ParseError", err, err)
	}
	if parseErr.Path != path || parseErr.Offset != 32 || parseErr.Expected != 32 || parseErr.Actual != 3 {
		t.Fatalf("parse error = %+v", parseErr)
	}
	msg := err.Error()
	for _, want := range []string{path, "offset=32", "expected=32", "actual=3"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}

func TestDailyPathRejectsInvalidSymbolCodes(t *testing.T) {
	reader, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	cases := []string{
		"",
		"   ",
		"../600000",
		"nested/../600000",
		"600/000",
		`600\000`,
		".",
		"..",
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			_, err := reader.dailyPath(model.Symbol{Market: model.MarketSH, Code: code})
			if err == nil {
				t.Fatal("dailyPath succeeded, want invalid symbol code error")
			}

			msg := err.Error()
			if !strings.Contains(msg, "symbol code") {
				t.Fatalf("error %q does not mention symbol code", msg)
			}
			if trimmed := strings.TrimSpace(code); trimmed == "" {
				if !strings.Contains(msg, "empty") {
					t.Fatalf("error %q does not mention empty code", msg)
				}
			} else if !strings.Contains(msg, trimmed) {
				t.Fatalf("error %q does not contain code %q", msg, trimmed)
			}
		})
	}
}

func dailyRecord(date uint32, open uint32, high uint32, low uint32, close uint32, amount float32, volume uint32, reserved uint32) []byte {
	record := make([]byte, 32)
	binary.LittleEndian.PutUint32(record[0:4], date)
	binary.LittleEndian.PutUint32(record[4:8], open)
	binary.LittleEndian.PutUint32(record[8:12], high)
	binary.LittleEndian.PutUint32(record[12:16], low)
	binary.LittleEndian.PutUint32(record[16:20], close)
	binary.LittleEndian.PutUint32(record[20:24], math.Float32bits(amount))
	binary.LittleEndian.PutUint32(record[24:28], volume)
	binary.LittleEndian.PutUint32(record[28:32], reserved)
	return record
}

func assertFloat(t *testing.T, name string, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
