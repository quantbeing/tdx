package vipdoc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestMinuteUnsupportedPeriod(t *testing.T) {
	reader, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	_, err = reader.Minute(context.Background(), model.Symbol{Market: model.MarketSH, Code: "600000"}, MinutePeriod(15))
	if !errors.Is(err, ErrUnsupportedPeriod) {
		t.Fatalf("Minute error = %v, want ErrUnsupportedPeriod", err)
	}
}

func TestMinuteOneAndFiveMinuteReturnUnsupportedFormat(t *testing.T) {
	reader, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	for _, period := range []MinutePeriod{MinutePeriod1, MinutePeriod5} {
		_, err = reader.Minute(context.Background(), model.Symbol{Market: model.MarketSH, Code: "600000"}, period)
		if !errors.Is(err, ErrUnsupportedFormat) {
			t.Fatalf("Minute(%v) error = %v, want ErrUnsupportedFormat", period, err)
		}
	}
}

func TestMinutePathRejectsInvalidSymbolCodes(t *testing.T) {
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
			_, err := reader.minutePath(model.Symbol{Market: model.MarketSH, Code: code}, MinutePeriod1)
			if err == nil {
				t.Fatal("minutePath succeeded, want invalid symbol code error")
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
