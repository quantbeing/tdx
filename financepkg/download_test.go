package financepkg

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestListPackagesParsesGPCWManifest(t *testing.T) {
	client := &memoryReportFileClient{
		files: map[string][]byte{
			"tdxfin/gpcw.txt": []byte("gpcw20240630.zip,abc123,42\n\n gpcw20240331.zip,def456,7 \n"),
		},
	}

	got, err := ListPackages(context.Background(), client)
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0] != (PackageMeta{Filename: "gpcw20240630.zip", Hash: "abc123", FileSize: 42}) {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[1] != (PackageMeta{Filename: "gpcw20240331.zip", Hash: "def456", FileSize: 7}) {
		t.Fatalf("got[1] = %+v", got[1])
	}
	if len(client.calls) != 1 || client.calls[0] != "tdxfin/gpcw.txt" {
		t.Fatalf("calls = %+v, want [tdxfin/gpcw.txt]", client.calls)
	}
}

func TestDownloadPackageReadsNamedFileFromTdxfin(t *testing.T) {
	want := []byte("zip bytes")
	client := &memoryReportFileClient{
		files: map[string][]byte{
			"tdxfin/gpcw20240630.zip": want,
		},
	}

	got, err := DownloadPackage(context.Background(), client, PackageMeta{Filename: "gpcw20240630.zip"})
	if err != nil {
		t.Fatalf("DownloadPackage: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("payload = %q, want %q", got, want)
	}
	if len(client.calls) != 1 || client.calls[0] != "tdxfin/gpcw20240630.zip" {
		t.Fatalf("calls = %+v, want tdxfin package path", client.calls)
	}
}

type memoryReportFileClient struct {
	files map[string][]byte
	calls []string
}

func (c *memoryReportFileClient) GetReportFile(_ context.Context, filename string) ([]byte, error) {
	c.calls = append(c.calls, filename)
	data, ok := c.files[filename]
	if !ok {
		return nil, errors.New("missing file")
	}
	return append([]byte(nil), data...), nil
}
