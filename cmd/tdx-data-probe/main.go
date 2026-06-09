package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quantbeing/tdx/diagnostic"
)

const defaultManifestURL = "https://data.tdx.com.cn/tdxgp/gpszsh.txt"
const defaultLocalIndexURL = "https://data.tdx.com.cn/tdxgp/gpszsh.local"
const defaultDat13URL = "https://data.tdx.com.cn/tdxgp/gpbj920021.dat"

type manifestProbeOutput struct {
	OK   bool   `json:"ok"`
	Kind string `json:"kind"`
	diagnostic.DataPackageManifestSummary
}

type localIndexProbeOutput struct {
	OK   bool   `json:"ok"`
	Kind string `json:"kind"`
	diagnostic.DataPackageLocalIndexSummary
}

type dat13ProbeOutput struct {
	OK   bool   `json:"ok"`
	Kind string `json:"kind"`
	diagnostic.DataPackageFixed13Summary
}

func main() {
	client := &http.Client{}
	if err := run(os.Args[1:], os.Stdout, client); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, client *http.Client) error {
	var kind string
	var url string
	var input string
	var prefix string
	var timeout time.Duration
	var limit int
	fs := flag.NewFlagSet("tdx-data-probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&kind, "kind", "manifest", "probe kind: manifest, local-index, or dat13")
	fs.StringVar(&url, "url", defaultManifestURL, "TDX data package URL")
	fs.StringVar(&input, "input", "", "optional local input file to parse instead of HTTP fetch")
	fs.StringVar(&prefix, "prefix", "", "optional filename prefix filter, for example gpbj")
	fs.DurationVar(&timeout, "timeout", 12*time.Second, "HTTP timeout")
	fs.IntVar(&limit, "limit", 20, "maximum entries to include in JSON output; -1 means all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else if client.Timeout == 0 {
		cloned := *client
		cloned.Timeout = timeout
		client = &cloned
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	kind = strings.ToLower(strings.TrimSpace(kind))
	if url == defaultManifestURL && kind == "local-index" {
		url = defaultLocalIndexURL
	}
	if url == defaultManifestURL && kind == "dat13" {
		url = defaultDat13URL
	}
	switch kind {
	case "manifest":
		return runManifestProbe(ctx, stdout, client, url, input, prefix, limit)
	case "local-index":
		return runLocalIndexProbe(ctx, stdout, client, url, input, prefix, limit)
	case "dat13":
		return runDat13Probe(ctx, stdout, client, url, input, limit)
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
}

func runManifestProbe(ctx context.Context, stdout io.Writer, client *http.Client, url string, input string, prefix string, limit int) error {
	manifest, err := loadManifest(ctx, client, url, input)
	if err != nil {
		return fmt.Errorf("probe data package manifest: %w", err)
	}
	if prefix != "" {
		manifest = diagnostic.FilterDataPackageManifestByPrefix(manifest, prefix)
	}
	out := manifestProbeOutput{
		OK:                         true,
		Kind:                       "manifest",
		DataPackageManifestSummary: diagnostic.SummarizeDataPackageManifest(manifest, limit),
	}
	return json.NewEncoder(stdout).Encode(out)
}

func runLocalIndexProbe(ctx context.Context, stdout io.Writer, client *http.Client, url string, input string, prefix string, limit int) error {
	index, err := loadLocalIndex(ctx, client, url, input)
	if err != nil {
		return fmt.Errorf("probe data package local index: %w", err)
	}
	if prefix != "" {
		index = diagnostic.FilterDataPackageLocalIndexByPrefix(index, prefix)
	}
	out := localIndexProbeOutput{
		OK:                           true,
		Kind:                         "local-index",
		DataPackageLocalIndexSummary: diagnostic.SummarizeDataPackageLocalIndex(index, limit),
	}
	return json.NewEncoder(stdout).Encode(out)
}

func runDat13Probe(ctx context.Context, stdout io.Writer, client *http.Client, url string, input string, limit int) error {
	records, err := loadDat13(ctx, client, url, input)
	if err != nil {
		return fmt.Errorf("probe data package dat13 records: %w", err)
	}
	out := dat13ProbeOutput{
		OK:                        true,
		Kind:                      "dat13",
		DataPackageFixed13Summary: diagnostic.SummarizeDataPackageFixed13Records(records, limit),
	}
	return json.NewEncoder(stdout).Encode(out)
}

func loadManifest(ctx context.Context, client *http.Client, url string, input string) (diagnostic.DataPackageManifest, error) {
	if input == "" {
		return diagnostic.FetchDataPackageManifest(ctx, url, client)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return diagnostic.DataPackageManifest{}, err
	}
	return diagnostic.ParseDataPackageManifest(input, data)
}

func loadLocalIndex(ctx context.Context, client *http.Client, url string, input string) (diagnostic.DataPackageLocalIndex, error) {
	if input == "" {
		return diagnostic.FetchDataPackageLocalIndex(ctx, url, client)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return diagnostic.DataPackageLocalIndex{}, err
	}
	return diagnostic.ParseDataPackageLocalIndex(input, data)
}

func loadDat13(ctx context.Context, client *http.Client, url string, input string) (diagnostic.DataPackageFixed13Records, error) {
	if input == "" {
		return diagnostic.FetchDataPackageFixed13Records(ctx, url, client)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return diagnostic.DataPackageFixed13Records{}, err
	}
	return diagnostic.ParseDataPackageFixed13Records(input, data)
}
