package diagnostic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDataPackageManifestParsesValidRowsAndRecordsSkippedRows(t *testing.T) {
	input := strings.Join([]string{
		"gpszsh.local,217d7299f6fca72f782c0144587c8533,354667",
		"",
		"broken,row",
		"gpsz302132.dat,778a287273c28e00579c4c9440e1d85c,249860",
		"gpcw20260331.zip,472ceae91c784d4a2dcbda37c1efde55,5041049",
	}, "\n")

	manifest, err := ParseDataPackageManifest("gpszsh.txt", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageManifest: %v", err)
	}

	if manifest.Source != "gpszsh.txt" {
		t.Fatalf("Source = %q", manifest.Source)
	}
	if len(manifest.Entries) != 3 {
		t.Fatalf("Entries = %+v", manifest.Entries)
	}
	if manifest.TotalSize != 5645576 {
		t.Fatalf("TotalSize = %d", manifest.TotalSize)
	}
	if manifest.LocalCount != 1 || manifest.DatCount != 1 || manifest.ZipCount != 1 {
		t.Fatalf("counts local/dat/zip = %d/%d/%d", manifest.LocalCount, manifest.DatCount, manifest.ZipCount)
	}
	if len(manifest.SkippedLines) != 1 || manifest.SkippedLines[0].Line != 3 {
		t.Fatalf("SkippedLines = %+v", manifest.SkippedLines)
	}
}

func TestDataPackageManifestFindAndFilter(t *testing.T) {
	input := strings.Join([]string{
		"gpszsh.local,217d7299f6fca72f782c0144587c8533,354667",
		"gpsz302132.dat,778a287273c28e00579c4c9440e1d85c,249860",
		"gpbj920021.dat,74b988089bee07374fe584e996b5483d,141154",
		"gpcw20260331.zip,472ceae91c784d4a2dcbda37c1efde55,5041049",
	}, "\n")

	manifest, err := ParseDataPackageManifest("fixture", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageManifest: %v", err)
	}

	entry, ok := manifest.FindEntry("gpsz302132.dat")
	if !ok || entry.Size != 249860 {
		t.Fatalf("FindEntry = %+v ok=%v", entry, ok)
	}
	if _, ok := manifest.FindEntry("missing.dat"); ok {
		t.Fatalf("FindEntry unexpectedly found missing entry")
	}
	if dat := manifest.EntriesByExtension(".dat"); len(dat) != 2 || dat[0].FileName != "gpsz302132.dat" {
		t.Fatalf("EntriesByExtension(.dat) = %+v", dat)
	}
	if bj := manifest.EntriesByPrefix("gpbj"); len(bj) != 1 || bj[0].FileName != "gpbj920021.dat" {
		t.Fatalf("EntriesByPrefix(gpbj) = %+v", bj)
	}
	if zip := manifest.EntriesByExtension("zip"); len(zip) != 1 || zip[0].FileName != "gpcw20260331.zip" {
		t.Fatalf("EntriesByExtension(zip) = %+v", zip)
	}
}

func TestFetchDataPackageManifestParsesHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tdxgp/gpszsh.txt" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("gpszsh.local,217d7299f6fca72f782c0144587c8533,354667\n"))
	}))
	defer server.Close()

	manifest, err := FetchDataPackageManifest(context.Background(), server.URL+"/tdxgp/gpszsh.txt", server.Client())
	if err != nil {
		t.Fatalf("FetchDataPackageManifest: %v", err)
	}
	if manifest.Source != server.URL+"/tdxgp/gpszsh.txt" || manifest.PackageCount != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestSummarizeDataPackageManifestLimitsEntries(t *testing.T) {
	input := strings.Join([]string{
		"gpszsh.local,217d7299f6fca72f782c0144587c8533,354667",
		"gpsz302132.dat,778a287273c28e00579c4c9440e1d85c,249860",
		"gpcw20260331.zip,472ceae91c784d4a2dcbda37c1efde55,5041049",
	}, "\n")
	manifest, err := ParseDataPackageManifest("fixture", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageManifest: %v", err)
	}

	summary := SummarizeDataPackageManifest(manifest, 2)
	if summary.EntryCount != 3 || len(summary.Entries) != 2 || summary.EntriesTruncated != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestFilterDataPackageManifestByPrefixKeepsSummaryConsistent(t *testing.T) {
	input := strings.Join([]string{
		"gpsz302132.dat,778a287273c28e00579c4c9440e1d85c,249860",
		"gpbj920021.dat,74b988089bee07374fe584e996b5483d,141154",
		"gpbj920020.dat,8afc95750292c3787fcd0026862afc71,115622",
	}, "\n")
	manifest, err := ParseDataPackageManifest("fixture", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageManifest: %v", err)
	}

	filtered := FilterDataPackageManifestByPrefix(manifest, "gpbj")
	if filtered.PackageCount != 2 || filtered.DatCount != 2 || filtered.TotalSize != 256776 {
		t.Fatalf("filtered = %+v", filtered)
	}
	if filtered.Source != "fixture" {
		t.Fatalf("Source = %q", filtered.Source)
	}
}

func TestParseDataPackageLocalIndexParsesMD5Section(t *testing.T) {
	input := strings.Join([]string{
		"[MD5]",
		"gpsz302132.dat=778a287273c28e00579c4c9440e1d85c",
		"broken",
		"gpbj920021.dat=74b988089bee07374fe584e996b5483d",
		"[OTHER]",
		"ignored.dat=0123456789abcdef0123456789abcdef",
	}, "\r\n")

	index, err := ParseDataPackageLocalIndex("gpszsh.local", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageLocalIndex: %v", err)
	}
	if index.Source != "gpszsh.local" || index.EntryCount != 2 || index.DatCount != 2 {
		t.Fatalf("index = %+v", index)
	}
	if len(index.SkippedLines) != 1 || index.SkippedLines[0].Line != 3 {
		t.Fatalf("SkippedLines = %+v", index.SkippedLines)
	}
	if bj := index.EntriesByPrefix("gpbj"); len(bj) != 1 || bj[0].FileName != "gpbj920021.dat" {
		t.Fatalf("EntriesByPrefix(gpbj) = %+v", bj)
	}
}

func TestFetchDataPackageLocalIndexParsesHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tdxgp/gpszsh.local" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("[MD5]\n"))
		_, _ = w.Write([]byte("gpbj920021.dat=74b988089bee07374fe584e996b5483d\n"))
	}))
	defer server.Close()

	index, err := FetchDataPackageLocalIndex(context.Background(), server.URL+"/tdxgp/gpszsh.local", server.Client())
	if err != nil {
		t.Fatalf("FetchDataPackageLocalIndex: %v", err)
	}
	if index.Source != server.URL+"/tdxgp/gpszsh.local" || index.EntryCount != 1 {
		t.Fatalf("index = %+v", index)
	}
}

func TestSummarizeAndFilterDataPackageLocalIndex(t *testing.T) {
	input := strings.Join([]string{
		"[MD5]",
		"gpsz302132.dat=778a287273c28e00579c4c9440e1d85c",
		"gpbj920021.dat=74b988089bee07374fe584e996b5483d",
		"gpbj920020.dat=8afc95750292c3787fcd0026862afc71",
	}, "\n")
	index, err := ParseDataPackageLocalIndex("fixture", []byte(input))
	if err != nil {
		t.Fatalf("ParseDataPackageLocalIndex: %v", err)
	}

	filtered := FilterDataPackageLocalIndexByPrefix(index, "gpbj")
	summary := SummarizeDataPackageLocalIndex(filtered, 1)
	if summary.EntryCount != 2 || len(summary.Entries) != 1 || summary.EntriesTruncated != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.DatCount != 2 || summary.Source != "fixture" {
		t.Fatalf("summary = %+v", summary)
	}
}

func BenchmarkParseDataPackageManifest(b *testing.B) {
	data := benchmarkDataPackageManifest(7240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		manifest, err := ParseDataPackageManifest("benchmark", data)
		if err != nil {
			b.Fatal(err)
		}
		if manifest.PackageCount != 7240 {
			b.Fatalf("PackageCount = %d", manifest.PackageCount)
		}
	}
}

func BenchmarkParseDataPackageLocalIndex(b *testing.B) {
	data := benchmarkDataPackageLocalIndex(7240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		index, err := ParseDataPackageLocalIndex("benchmark", data)
		if err != nil {
			b.Fatal(err)
		}
		if index.EntryCount != 7240 {
			b.Fatalf("EntryCount = %d", index.EntryCount)
		}
	}
}

func benchmarkDataPackageManifest(count int) []byte {
	var b strings.Builder
	for i := 0; i < count; i++ {
		prefix := "gpsz"
		if i%7 == 0 {
			prefix = "gpbj"
		}
		_, _ = fmt.Fprintf(&b, "%s%06d.dat,0123456789abcdef0123456789abcdef,%d\n", prefix, i, 1000+i)
	}
	return []byte(b.String())
}

func benchmarkDataPackageLocalIndex(count int) []byte {
	var b strings.Builder
	b.WriteString("[MD5]\n")
	for i := 0; i < count; i++ {
		prefix := "gpsz"
		if i%7 == 0 {
			prefix = "gpbj"
		}
		_, _ = fmt.Fprintf(&b, "%s%06d.dat=0123456789abcdef0123456789abcdef\n", prefix, i)
	}
	return []byte(b.String())
}
