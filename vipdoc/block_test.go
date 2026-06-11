package vipdoc

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockFileParsesFlatMembersAndPreservesIndexes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vipdoc", "sh", "block", "block_gn.dat")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data := blockFile(blockRecord([]byte{0xc8, 0xcb, 0xb9, 0xa4, 0xd6, 0xc7, 0xc4, 0xdc}, 2, 2, "600519", "000001"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	members, err := reader.BlockFile(context.Background(), "block_gn.dat")
	if err != nil {
		t.Fatalf("BlockFile: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("len(members) = %d, want 2", len(members))
	}
	first := members[0]
	if first.Filename != "block_gn.dat" || first.BlockIndex != 0 || first.BlockName != "人工智能" ||
		first.BlockType != 2 || first.StockCount != 2 || first.CodeIndex != 0 || first.Code != "600519" {
		t.Fatalf("first member = %+v", first)
	}
	if len(first.RawCode) != 7 || len(first.RawBlock) != 2813 {
		t.Fatalf("raw lengths = code %d block %d, want 7/2813", len(first.RawCode), len(first.RawBlock))
	}
	second := members[1]
	if second.CodeIndex != 1 || second.Code != "000001" || second.BlockName != first.BlockName {
		t.Fatalf("second member = %+v", second)
	}
}

func TestBlockFileTruncatedBlockReportsPathOffsetExpectedAndActual(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vipdoc", "sh", "block", "block_gn.dat")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data := make([]byte, 384)
	data = binary.LittleEndian.AppendUint16(data, 1)
	data = append(data, make([]byte, 20)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = reader.BlockFile(context.Background(), "block_gn.dat")
	if err == nil {
		t.Fatal("BlockFile succeeded, want truncation error")
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error %T = %v, want *ParseError", err, err)
	}
	if parseErr.Path != path || parseErr.Offset != 386 || parseErr.Expected != 2813 || parseErr.Actual != 20 {
		t.Fatalf("parse error = %+v", parseErr)
	}
	msg := err.Error()
	for _, want := range []string{path, "offset=386", "expected=2813", "actual=20"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}

func TestBlockFileUnsupportedStockCountReportsStockCountAndMax(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "vipdoc", "sh", "block", "block_gn.dat")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data := blockFile(blockRecord(nil, uint16(maxBlockCodes+1), 2))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = reader.BlockFile(context.Background(), "block_gn.dat")
	if err == nil {
		t.Fatal("BlockFile succeeded, want unsupported format error")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("BlockFile error = %v, want ErrUnsupportedFormat", err)
	}
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		t.Fatalf("BlockFile error = %T, want non-ParseError unsupported format", err)
	}
	msg := err.Error()
	for _, want := range []string{path, "stock_count=401", "max=400"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}

func blockFile(records ...[]byte) []byte {
	data := make([]byte, 384)
	data = binary.LittleEndian.AppendUint16(data, uint16(len(records)))
	for _, record := range records {
		data = append(data, record...)
	}
	return data
}

func blockRecord(name []byte, stockCount uint16, blockType uint16, codes ...string) []byte {
	record := make([]byte, 2813)
	copy(record[0:9], name)
	binary.LittleEndian.PutUint16(record[9:11], stockCount)
	binary.LittleEndian.PutUint16(record[11:13], blockType)
	for i, code := range codes {
		start := 13 + i*7
		copy(record[start:start+7], []byte(code))
	}
	return record
}
