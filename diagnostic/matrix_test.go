package diagnostic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

type fakeCapturer struct {
	fail map[string]error
}

func (f fakeCapturer) Capture(_ context.Context, cmd command.Command) (tdx.CapturedResponse, error) {
	if err := f.fail[cmd.Operation()]; err != nil {
		return tdx.CapturedResponse{}, err
	}
	return tdx.CapturedResponse{
		Operation:   cmd.Operation(),
		Server:      model.Server{Name: "fake", Host: "127.0.0.1", Port: 7709},
		Attempt:     1,
		Latency:     time.Millisecond,
		Request:     []byte{1, 2},
		Header:      frame.Header{ZipSize: 2, UnzipSize: 2},
		HeaderBytes: make([]byte, frame.HeaderSize),
		RawBody:     []byte{3, 4},
		Body:        []byte{3, 4},
		Parsed:      map[string]any{"operation": cmd.Operation()},
	}, nil
}

func TestRunCaptureMatrixWritesFixturesAndContinuesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	results := RunCaptureMatrix(context.Background(), fakeCapturer{
		fail: map[string]error{"security_list": errors.New("timeout")},
	}, dir, []MatrixOperation{
		{Name: "count", Command: command.NewSecurityCountCommand(model.MarketSH)},
		{Name: "list", Command: command.NewSecurityListCommand(model.MarketSH, 0)},
	})

	if len(results) != 2 {
		t.Fatalf("len = %d", len(results))
	}
	if !results[0].OK || results[0].Path == "" || results[0].BodySize != 2 {
		t.Fatalf("first result = %+v", results[0])
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("fixture not written: %v", err)
	}
	if results[1].OK || results[1].Error == "" || results[1].Path != "" {
		t.Fatalf("second result = %+v", results[1])
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*.fixture.json")); err != nil || len(matches) != 1 {
		t.Fatalf("fixtures = %v err=%v", matches, err)
	}
}

func TestSelectMatrixOperationsKeepsRequestedOrder(t *testing.T) {
	selected, unknown := SelectMatrixOperations(DefaultMatrixOperations(), []string{"quote", "security-count"})
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v", unknown)
	}
	if len(selected) != 2 || selected[0].Name != "quote" || selected[1].Name != "security-count" {
		t.Fatalf("selected = %+v", selected)
	}

	_, unknown = SelectMatrixOperations(DefaultMatrixOperations(), []string{"missing"})
	if len(unknown) != 1 || unknown[0] != "missing" {
		t.Fatalf("unknown = %v", unknown)
	}
}
