package vipdoc

import (
	"errors"
	"strings"
	"testing"
)

func TestParseErrorIncludesDiagnosticsAndUnwraps(t *testing.T) {
	err := &ParseError{
		Path:     "/tmp/bad.day",
		Offset:   64,
		Expected: 32,
		Actual:   5,
		Err:      ErrTruncatedFile,
	}

	if !errors.Is(err, ErrTruncatedFile) {
		t.Fatal("ParseError does not unwrap ErrTruncatedFile")
	}
	msg := err.Error()
	for _, want := range []string{"/tmp/bad.day", "offset=64", "expected=32", "actual=5"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}
