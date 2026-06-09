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
	var prefix string
	var timeout time.Duration
	var limit int
	fs := flag.NewFlagSet("tdx-data-probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&kind, "kind", "manifest", "probe kind: manifest or local-index")
	fs.StringVar(&url, "url", defaultManifestURL, "TDX data package URL")
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
	switch kind {
	case "manifest":
		return runManifestProbe(ctx, stdout, client, url, prefix, limit)
	case "local-index":
		return runLocalIndexProbe(ctx, stdout, client, url, prefix, limit)
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
}

func runManifestProbe(ctx context.Context, stdout io.Writer, client *http.Client, url string, prefix string, limit int) error {
	manifest, err := diagnostic.FetchDataPackageManifest(ctx, url, client)
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

func runLocalIndexProbe(ctx context.Context, stdout io.Writer, client *http.Client, url string, prefix string, limit int) error {
	index, err := diagnostic.FetchDataPackageLocalIndex(ctx, url, client)
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
