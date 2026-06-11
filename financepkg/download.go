package financepkg

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ReportFileClient interface {
	GetReportFile(ctx context.Context, filename string) ([]byte, error)
}

type PackageMeta struct {
	Filename string
	Hash     string
	FileSize int
}

func ListPackages(ctx context.Context, client ReportFileClient) ([]PackageMeta, error) {
	data, err := client.GetReportFile(ctx, "tdxfin/gpcw.txt")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]PackageMeta, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("finance package manifest line %d: expected 3 fields, actual %d", i+1, len(parts))
		}
		size, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return nil, fmt.Errorf("finance package manifest line %d filesize: %w", i+1, err)
		}
		out = append(out, PackageMeta{
			Filename: strings.TrimSpace(parts[0]),
			Hash:     strings.TrimSpace(parts[1]),
			FileSize: size,
		})
	}
	return out, nil
}

func DownloadPackage(ctx context.Context, client ReportFileClient, meta PackageMeta) ([]byte, error) {
	return client.GetReportFile(ctx, "tdxfin/"+meta.Filename)
}
