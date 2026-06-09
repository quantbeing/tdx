package diagnostic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

func TestWriteFixtureEncodesFrameBytesAndParsedJSON(t *testing.T) {
	capture := tdx.CapturedResponse{
		Operation:   "security_count",
		Server:      model.Server{Name: "fake", Host: "127.0.0.1", Port: 7709},
		Request:     []byte{0x01, 0x02},
		Header:      frame.Header{ZipSize: 2, UnzipSize: 2},
		HeaderBytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 2, 0},
		RawBody:     []byte{0xd2, 0x04},
		Body:        []byte{0xd2, 0x04},
		Parsed:      uint16(1234),
	}
	path := filepath.Join(t.TempDir(), "security_count.fixture.json")

	summary, err := WriteFixture(path, capture)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	if summary.Path != path || summary.BodySize != 2 || summary.RawBodySize != 2 {
		t.Fatalf("summary = %+v", summary)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if fixture.RequestHex != "0102" || fixture.BodyHex != "d204" || fixture.RawBodyHex != "d204" {
		t.Fatalf("hex fields = %q/%q/%q", fixture.RequestHex, fixture.RawBodyHex, fixture.BodyHex)
	}
	if string(fixture.ParsedJSON) != "1234" {
		t.Fatalf("parsed json = %s", fixture.ParsedJSON)
	}
}

func TestCompareJSONReportsPathLevelMismatches(t *testing.T) {
	report, err := CompareJSON(
		[]byte(`{"price":10,"rows":[{"code":"600519"}]}`),
		[]byte(`{"price":11,"rows":[{"code":"600519"}],"extra":true}`),
		CompareOptions{},
	)
	if err != nil {
		t.Fatalf("CompareJSON: %v", err)
	}
	if report.Equal || len(report.Diffs) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if !hasDiffPath(report.Diffs, "$.price") || !hasDiffPath(report.Diffs, "$.extra") {
		t.Fatalf("diffs = %+v", report.Diffs)
	}
}

func hasDiffPath(diffs []Diff, path string) bool {
	for _, diff := range diffs {
		if diff.Path == path {
			return true
		}
	}
	return false
}
