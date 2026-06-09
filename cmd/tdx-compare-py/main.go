package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/quantbeing/tdx/diagnostic"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer) error {
	var goPath string
	var pyPath string
	var maxDiffs int
	var tolerance float64
	fs := flag.NewFlagSet("tdx-compare-py", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&goPath, "go", "", "Go result JSON or Go fixture JSON")
	fs.StringVar(&pyPath, "py", "", "Python reference JSON")
	fs.IntVar(&maxDiffs, "max-diffs", 100, "maximum diffs to report")
	fs.Float64Var(&tolerance, "tolerance", 0, "numeric tolerance")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if goPath == "" || pyPath == "" {
		return fmt.Errorf("usage: tdx-compare-py -go go.json -py py.json [-max-diffs 100] [-tolerance 0.0001]")
	}
	report, err := compareFiles(goPath, pyPath, diagnostic.CompareOptions{MaxDiffs: maxDiffs, NumericTolerance: tolerance})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func compareFiles(goPath string, pyPath string, opts diagnostic.CompareOptions) (diagnostic.ComparisonReport, error) {
	goJSON, err := loadComparableJSON(goPath)
	if err != nil {
		return diagnostic.ComparisonReport{}, err
	}
	pyJSON, err := loadComparableJSON(pyPath)
	if err != nil {
		return diagnostic.ComparisonReport{}, err
	}
	return diagnostic.CompareJSON(goJSON, pyJSON, opts)
}

func loadComparableJSON(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fixture struct {
		ParsedJSON json.RawMessage `json:"parsed_json"`
	}
	if err := json.Unmarshal(raw, &fixture); err == nil && len(fixture.ParsedJSON) > 0 && string(fixture.ParsedJSON) != "null" {
		return fixture.ParsedJSON, nil
	}
	return raw, nil
}
