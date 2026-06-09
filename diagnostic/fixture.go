package diagnostic

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

type Fixture struct {
	SchemaVersion int             `json:"schema_version"`
	CapturedAt    time.Time       `json:"captured_at"`
	Operation     string          `json:"operation"`
	Server        model.Server    `json:"server"`
	Attempt       int             `json:"attempt"`
	LatencyMS     int64           `json:"latency_ms"`
	Header        frame.Header    `json:"header"`
	RequestHex    string          `json:"request_hex"`
	HeaderHex     string          `json:"header_hex"`
	RawBodyHex    string          `json:"raw_body_hex"`
	BodyHex       string          `json:"body_hex"`
	RawBodySize   int             `json:"raw_body_size"`
	BodySize      int             `json:"body_size"`
	ParsedJSON    json.RawMessage `json:"parsed_json,omitempty"`
}

type FixtureSummary struct {
	Path        string       `json:"path"`
	Operation   string       `json:"operation"`
	Server      model.Server `json:"server"`
	RawBodySize int          `json:"raw_body_size"`
	BodySize    int          `json:"body_size"`
}

func NewFixture(capture tdx.CapturedResponse) (Fixture, error) {
	parsed, err := json.Marshal(capture.Parsed)
	if err != nil {
		return Fixture{}, fmt.Errorf("marshal parsed response: %w", err)
	}
	return Fixture{
		SchemaVersion: 1,
		CapturedAt:    time.Now().UTC(),
		Operation:     capture.Operation,
		Server:        capture.Server,
		Attempt:       capture.Attempt,
		LatencyMS:     capture.Latency.Milliseconds(),
		Header:        capture.Header,
		RequestHex:    hex.EncodeToString(capture.Request),
		HeaderHex:     hex.EncodeToString(capture.HeaderBytes),
		RawBodyHex:    hex.EncodeToString(capture.RawBody),
		BodyHex:       hex.EncodeToString(capture.Body),
		RawBodySize:   len(capture.RawBody),
		BodySize:      len(capture.Body),
		ParsedJSON:    json.RawMessage(parsed),
	}, nil
}

func WriteFixture(path string, capture tdx.CapturedResponse) (FixtureSummary, error) {
	fixture, err := NewFixture(capture)
	if err != nil {
		return FixtureSummary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FixtureSummary{}, err
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return FixtureSummary{}, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return FixtureSummary{}, err
	}
	return FixtureSummary{
		Path:        path,
		Operation:   fixture.Operation,
		Server:      fixture.Server,
		RawBodySize: fixture.RawBodySize,
		BodySize:    fixture.BodySize,
	}, nil
}

func DefaultFixturePath(dir string, capture tdx.CapturedResponse) string {
	ts := time.Now().UTC().Format("20060102T150405.000000000Z")
	host := strings.NewReplacer(":", "_", ".", "_", "/", "_").Replace(capture.Server.Addr())
	name := fmt.Sprintf("%s_%s_%s.fixture.json", sanitizeName(capture.Operation), host, ts)
	return filepath.Join(dir, name)
}

type CompareOptions struct {
	MaxDiffs         int     `json:"max_diffs,omitempty"`
	NumericTolerance float64 `json:"numeric_tolerance,omitempty"`
}

type Diff struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	GoValue     any    `json:"go_value,omitempty"`
	PythonValue any    `json:"python_value,omitempty"`
}

type ComparisonReport struct {
	Equal bool   `json:"equal"`
	Diffs []Diff `json:"diffs,omitempty"`
}

func CompareJSON(goJSON []byte, pythonJSON []byte, opts CompareOptions) (ComparisonReport, error) {
	var goValue any
	if err := decodeJSON(goJSON, &goValue); err != nil {
		return ComparisonReport{}, fmt.Errorf("decode go json: %w", err)
	}
	var pyValue any
	if err := decodeJSON(pythonJSON, &pyValue); err != nil {
		return ComparisonReport{}, fmt.Errorf("decode python json: %w", err)
	}
	diffs := make([]Diff, 0)
	compareValue("$", goValue, pyValue, opts, &diffs)
	return ComparisonReport{Equal: len(diffs) == 0, Diffs: diffs}, nil
}

func decodeJSON(data []byte, out *any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(out)
}

func compareValue(path string, goValue any, pyValue any, opts CompareOptions, diffs *[]Diff) {
	if opts.MaxDiffs > 0 && len(*diffs) >= opts.MaxDiffs {
		return
	}
	switch g := goValue.(type) {
	case map[string]any:
		p, ok := pyValue.(map[string]any)
		if !ok {
			addDiff(path, "type_mismatch", goValue, pyValue, opts, diffs)
			return
		}
		keys := make([]string, 0, len(g)+len(p))
		seen := make(map[string]struct{}, len(g)+len(p))
		for key := range g {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		for key := range p {
			if _, ok := seen[key]; !ok {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := path + "." + key
			gv, gok := g[key]
			pv, pok := p[key]
			switch {
			case !gok:
				addDiff(childPath, "missing_in_go", nil, pv, opts, diffs)
			case !pok:
				addDiff(childPath, "missing_in_python", gv, nil, opts, diffs)
			default:
				compareValue(childPath, gv, pv, opts, diffs)
			}
		}
	case []any:
		p, ok := pyValue.([]any)
		if !ok {
			addDiff(path, "type_mismatch", goValue, pyValue, opts, diffs)
			return
		}
		if len(g) != len(p) {
			addDiff(path, "length_mismatch", len(g), len(p), opts, diffs)
		}
		n := len(g)
		if len(p) < n {
			n = len(p)
		}
		for i := 0; i < n; i++ {
			compareValue(fmt.Sprintf("%s[%d]", path, i), g[i], p[i], opts, diffs)
		}
	default:
		if numbersEqual(goValue, pyValue, opts.NumericTolerance) {
			return
		}
		if !reflect.DeepEqual(goValue, pyValue) {
			addDiff(path, "value_mismatch", goValue, pyValue, opts, diffs)
		}
	}
}

func addDiff(path string, kind string, goValue any, pyValue any, opts CompareOptions, diffs *[]Diff) {
	if opts.MaxDiffs > 0 && len(*diffs) >= opts.MaxDiffs {
		return
	}
	*diffs = append(*diffs, Diff{Path: path, Kind: kind, GoValue: goValue, PythonValue: pyValue})
}

func numbersEqual(a any, b any, tolerance float64) bool {
	af, aok := numberFloat(a)
	bf, bok := numberFloat(b)
	if !aok || !bok {
		return false
	}
	return math.Abs(af-bf) <= tolerance
}

func numberFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func sanitizeName(raw string) string {
	if raw == "" {
		return "tdx"
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
