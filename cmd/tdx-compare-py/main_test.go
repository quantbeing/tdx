package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quantbeing/tdx/diagnostic"
)

func TestCompareFilesUsesFixtureParsedJSON(t *testing.T) {
	dir := t.TempDir()
	goPath := filepath.Join(dir, "go.fixture.json")
	pyPath := filepath.Join(dir, "py.json")
	if err := os.WriteFile(goPath, []byte(`{"schema_version":1,"operation":"quote","parsed_json":{"price":10,"code":"600519"}}`), 0o644); err != nil {
		t.Fatalf("write go fixture: %v", err)
	}
	if err := os.WriteFile(pyPath, []byte(`{"price":11,"code":"600519"}`), 0o644); err != nil {
		t.Fatalf("write py json: %v", err)
	}

	report, err := compareFiles(goPath, pyPath, diagnostic.CompareOptions{})
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	if report.Equal || len(report.Diffs) != 1 || report.Diffs[0].Path != "$.price" {
		t.Fatalf("report = %+v", report)
	}
}
