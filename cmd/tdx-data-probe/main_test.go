package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesDataPackageSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("gpszsh.local,217d7299f6fca72f782c0144587c8533,354667\n"))
		_, _ = w.Write([]byte("gpsz302132.dat,778a287273c28e00579c4c9440e1d85c,249860\n"))
		_, _ = w.Write([]byte("gpbj920021.dat,74b988089bee07374fe584e996b5483d,141154\n"))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := run([]string{"-url", server.URL + "/tdxgp/gpszsh.txt", "-prefix", "gpbj", "-limit", "1"}, &out, server.Client()); err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if result["ok"] != true || result["entry_count"].(float64) != 1 {
		t.Fatalf("result = %+v", result)
	}
	entries := result["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	entry := entries[0].(map[string]any)
	if entry["file_name"] != "gpbj920021.dat" {
		t.Fatalf("entry = %+v", entry)
	}
	if result["entries_truncated"].(float64) != 0 {
		t.Fatalf("entries_truncated = %+v", result["entries_truncated"])
	}
}

func TestRunWritesLocalIndexSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[MD5]\n"))
		_, _ = w.Write([]byte("gpsz302132.dat=778a287273c28e00579c4c9440e1d85c\n"))
		_, _ = w.Write([]byte("gpbj920021.dat=74b988089bee07374fe584e996b5483d\n"))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := run([]string{"-kind", "local-index", "-url", server.URL + "/tdxgp/gpszsh.local", "-prefix", "gpbj", "-limit", "5"}, &out, server.Client()); err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if result["ok"] != true || result["kind"] != "local-index" || result["entry_count"].(float64) != 1 {
		t.Fatalf("result = %+v", result)
	}
	entries := result["entries"].([]any)
	entry := entries[0].(map[string]any)
	if entry["file_name"] != "gpbj920021.dat" {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestRunWritesDat13Summary(t *testing.T) {
	payload, err := hex.DecodeString("01bf7b330100008841000000000176a033010000104200000000")
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := run([]string{"-kind", "dat13", "-url", server.URL + "/tdxgp/gpbj920021.dat", "-limit", "1"}, &out, server.Client()); err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if result["ok"] != true || result["kind"] != "dat13" || result["record_count"].(float64) != 2 {
		t.Fatalf("result = %+v", result)
	}
	records := result["records"].([]any)
	if len(records) != 1 {
		t.Fatalf("records = %+v", records)
	}
	first := records[0].(map[string]any)
	if first["date_like"].(float64) != 20151231 || first["field1_float32"].(float64) != 17 {
		t.Fatalf("first = %+v", first)
	}
}

func TestRunWritesDat13SummaryFromInputFile(t *testing.T) {
	payload, err := hex.DecodeString("01bf7b33010000884100000000")
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	path := filepath.Join(t.TempDir(), "gpbj920021.dat")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var out bytes.Buffer
	if err := run([]string{"-kind", "dat13", "-input", path, "-limit", "1"}, &out, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if result["source"] != path || result["record_count"].(float64) != 1 {
		t.Fatalf("result = %+v", result)
	}
}
