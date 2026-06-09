package main

import (
	"os"
	"strings"
	"testing"

	tdx "github.com/quantbeing/tdx"
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
