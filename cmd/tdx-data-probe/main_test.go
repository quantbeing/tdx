package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
