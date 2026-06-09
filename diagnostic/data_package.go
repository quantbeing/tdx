package diagnostic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

const maxDataPackageBytes int64 = 64 * 1024 * 1024

type DataPackageEntry struct {
	FileName string `json:"file_name"`
	MD5      string `json:"md5"`
	Size     int64  `json:"size"`
	Line     int    `json:"line"`
}

type DataPackageChecksumEntry struct {
	FileName string `json:"file_name"`
	MD5      string `json:"md5"`
	Line     int    `json:"line"`
}

type DataPackageSkippedLine struct {
	Line   int    `json:"line"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

type DataPackageManifest struct {
	Source       string                   `json:"source,omitempty"`
	Entries      []DataPackageEntry       `json:"entries"`
	TotalSize    int64                    `json:"total_size"`
	PackageCount int                      `json:"package_count"`
	LocalCount   int                      `json:"local_count"`
	DatCount     int                      `json:"dat_count"`
	ZipCount     int                      `json:"zip_count"`
	OtherCount   int                      `json:"other_count"`
	SkippedLines []DataPackageSkippedLine `json:"skipped_lines,omitempty"`
}

type DataPackageLocalIndex struct {
	Source       string                     `json:"source,omitempty"`
	Entries      []DataPackageChecksumEntry `json:"entries"`
	EntryCount   int                        `json:"entry_count"`
	DatCount     int                        `json:"dat_count"`
	OtherCount   int                        `json:"other_count"`
	SkippedLines []DataPackageSkippedLine   `json:"skipped_lines,omitempty"`
}

type DataPackageFixed13Record struct {
	Offset             int     `json:"offset"`
	RawHex             string  `json:"raw_hex"`
	Marker             uint8   `json:"marker"`
	DateLike           int     `json:"date_like"`
	Field1Bits         uint32  `json:"field1_bits"`
	Field1Float32      float64 `json:"field1_float32"`
	Field1Float32Valid bool    `json:"field1_float32_valid"`
	Field2Uint32       uint32  `json:"field2_uint32"`
}

type DataPackageFixed13Records struct {
	Source           string                     `json:"source,omitempty"`
	RecordSize       int                        `json:"record_size"`
	Records          []DataPackageFixed13Record `json:"records"`
	RecordCount      int                        `json:"record_count"`
	TrailingBytes    int                        `json:"trailing_bytes"`
	TrailingBytesHex string                     `json:"trailing_bytes_hex,omitempty"`
}

type DataPackageManifestSummary struct {
	Source           string             `json:"source,omitempty"`
	EntryCount       int                `json:"entry_count"`
	TotalSize        int64              `json:"total_size"`
	LocalCount       int                `json:"local_count"`
	DatCount         int                `json:"dat_count"`
	ZipCount         int                `json:"zip_count"`
	OtherCount       int                `json:"other_count"`
	SkippedCount     int                `json:"skipped_count"`
	EntriesTruncated int                `json:"entries_truncated"`
	Entries          []DataPackageEntry `json:"entries,omitempty"`
}

type DataPackageLocalIndexSummary struct {
	Source           string                     `json:"source,omitempty"`
	EntryCount       int                        `json:"entry_count"`
	DatCount         int                        `json:"dat_count"`
	OtherCount       int                        `json:"other_count"`
	SkippedCount     int                        `json:"skipped_count"`
	EntriesTruncated int                        `json:"entries_truncated"`
	Entries          []DataPackageChecksumEntry `json:"entries,omitempty"`
}

type DataPackageFixed13Summary struct {
	Source             string                     `json:"source,omitempty"`
	RecordSize         int                        `json:"record_size"`
	RecordCount        int                        `json:"record_count"`
	TrailingBytes      int                        `json:"trailing_bytes"`
	TrailingBytesHex   string                     `json:"trailing_bytes_hex,omitempty"`
	MarkerCounts       map[string]int             `json:"marker_counts,omitempty"`
	DateLikeMin        int                        `json:"date_like_min"`
	DateLikeMax        int                        `json:"date_like_max"`
	Field1Float32Min   float64                    `json:"field1_float32_min"`
	Field1Float32Max   float64                    `json:"field1_float32_max"`
	Field2NonzeroCount int                        `json:"field2_nonzero_count"`
	RecordsTruncated   int                        `json:"records_truncated"`
	Records            []DataPackageFixed13Record `json:"records,omitempty"`
}

func FetchDataPackageManifest(ctx context.Context, url string, client *http.Client) (DataPackageManifest, error) {
	data, err := fetchDataPackageBytes(ctx, url, client)
	if err != nil {
		return DataPackageManifest{}, err
	}
	return ParseDataPackageManifest(url, data)
}

func FetchDataPackageLocalIndex(ctx context.Context, url string, client *http.Client) (DataPackageLocalIndex, error) {
	data, err := fetchDataPackageBytes(ctx, url, client)
	if err != nil {
		return DataPackageLocalIndex{}, err
	}
	return ParseDataPackageLocalIndex(url, data)
}

func FetchDataPackageFixed13Records(ctx context.Context, url string, client *http.Client) (DataPackageFixed13Records, error) {
	data, err := fetchDataPackageBytes(ctx, url, client)
	if err != nil {
		return DataPackageFixed13Records{}, err
	}
	return ParseDataPackageFixed13Records(url, data)
}

func fetchDataPackageBytes(ctx context.Context, url string, client *http.Client) ([]byte, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("data package url is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build data package request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "tdx-go/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data package: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch data package: status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDataPackageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read data package: %w", err)
	}
	if int64(len(data)) > maxDataPackageBytes {
		return nil, fmt.Errorf("read data package: response exceeds %d bytes", maxDataPackageBytes)
	}
	if isHTMLChallengeResponse(resp.Header.Get("Content-Type"), data) {
		return nil, fmt.Errorf("fetch data package: response looks like html/javascript challenge content_type=%q", resp.Header.Get("Content-Type"))
	}
	return data, nil
}

func SummarizeDataPackageManifest(manifest DataPackageManifest, limit int) DataPackageManifestSummary {
	if limit < 0 || limit > len(manifest.Entries) {
		limit = len(manifest.Entries)
	}
	entries := make([]DataPackageEntry, 0, limit)
	if limit > 0 {
		entries = append(entries, manifest.Entries[:limit]...)
	}
	return DataPackageManifestSummary{
		Source:           manifest.Source,
		EntryCount:       len(manifest.Entries),
		TotalSize:        manifest.TotalSize,
		LocalCount:       manifest.LocalCount,
		DatCount:         manifest.DatCount,
		ZipCount:         manifest.ZipCount,
		OtherCount:       manifest.OtherCount,
		SkippedCount:     len(manifest.SkippedLines),
		EntriesTruncated: len(manifest.Entries) - len(entries),
		Entries:          entries,
	}
}

func SummarizeDataPackageLocalIndex(index DataPackageLocalIndex, limit int) DataPackageLocalIndexSummary {
	if limit < 0 || limit > len(index.Entries) {
		limit = len(index.Entries)
	}
	entries := make([]DataPackageChecksumEntry, 0, limit)
	if limit > 0 {
		entries = append(entries, index.Entries[:limit]...)
	}
	return DataPackageLocalIndexSummary{
		Source:           index.Source,
		EntryCount:       len(index.Entries),
		DatCount:         index.DatCount,
		OtherCount:       index.OtherCount,
		SkippedCount:     len(index.SkippedLines),
		EntriesTruncated: len(index.Entries) - len(entries),
		Entries:          entries,
	}
}

func SummarizeDataPackageFixed13Records(records DataPackageFixed13Records, limit int) DataPackageFixed13Summary {
	if limit < 0 || limit > len(records.Records) {
		limit = len(records.Records)
	}
	rows := make([]DataPackageFixed13Record, 0, limit)
	if limit > 0 {
		rows = append(rows, records.Records[:limit]...)
	}
	stats := summarizeFixed13Stats(records.Records)
	return DataPackageFixed13Summary{
		Source:             records.Source,
		RecordSize:         records.RecordSize,
		RecordCount:        len(records.Records),
		TrailingBytes:      records.TrailingBytes,
		TrailingBytesHex:   records.TrailingBytesHex,
		MarkerCounts:       stats.markerCounts,
		DateLikeMin:        stats.dateLikeMin,
		DateLikeMax:        stats.dateLikeMax,
		Field1Float32Min:   stats.field1Min,
		Field1Float32Max:   stats.field1Max,
		Field2NonzeroCount: stats.field2NonzeroCount,
		RecordsTruncated:   len(records.Records) - len(rows),
		Records:            rows,
	}
}

func ParseDataPackageManifest(source string, data []byte) (DataPackageManifest, error) {
	manifest := DataPackageManifest{Source: source}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		entry, reason, ok := parseDataPackageManifestLine(line, lineNo)
		if !ok {
			manifest.SkippedLines = append(manifest.SkippedLines, DataPackageSkippedLine{
				Line:   lineNo,
				Raw:    rawLine,
				Reason: reason,
			})
			continue
		}
		addDataPackageManifestEntry(&manifest, entry)
	}
	if err := scanner.Err(); err != nil {
		return DataPackageManifest{}, fmt.Errorf("scan data package manifest: %w", err)
	}
	return manifest, nil
}

func ParseDataPackageFixed13Records(source string, data []byte) (DataPackageFixed13Records, error) {
	const recordSize = 13
	out := DataPackageFixed13Records{
		Source:     source,
		RecordSize: recordSize,
	}
	fullSize := len(data) / recordSize * recordSize
	out.Records = make([]DataPackageFixed13Record, 0, fullSize/recordSize)
	for offset := 0; offset < fullSize; offset += recordSize {
		raw := data[offset : offset+recordSize]
		field1Bits := binary.LittleEndian.Uint32(raw[5:9])
		field1 := float64(math.Float32frombits(field1Bits))
		field1Valid := !math.IsNaN(field1) && !math.IsInf(field1, 0)
		if !field1Valid {
			field1 = 0
		}
		out.Records = append(out.Records, DataPackageFixed13Record{
			Offset:             offset,
			RawHex:             hex.EncodeToString(raw),
			Marker:             raw[0],
			DateLike:           int(binary.LittleEndian.Uint32(raw[1:5])),
			Field1Bits:         field1Bits,
			Field1Float32:      field1,
			Field1Float32Valid: field1Valid,
			Field2Uint32:       binary.LittleEndian.Uint32(raw[9:13]),
		})
	}
	out.RecordCount = len(out.Records)
	if fullSize < len(data) {
		trailing := data[fullSize:]
		out.TrailingBytes = len(trailing)
		out.TrailingBytesHex = hex.EncodeToString(trailing)
	}
	return out, nil
}

type fixed13Stats struct {
	markerCounts       map[string]int
	dateLikeMin        int
	dateLikeMax        int
	field1Min          float64
	field1Max          float64
	field1Seen         bool
	field2NonzeroCount int
}

func summarizeFixed13Stats(records []DataPackageFixed13Record) fixed13Stats {
	stats := fixed13Stats{markerCounts: make(map[string]int)}
	if len(records) == 0 {
		return stats
	}
	stats.dateLikeMin = records[0].DateLike
	stats.dateLikeMax = records[0].DateLike
	for _, record := range records {
		stats.markerCounts[strconv.Itoa(int(record.Marker))]++
		if record.DateLike < stats.dateLikeMin {
			stats.dateLikeMin = record.DateLike
		}
		if record.DateLike > stats.dateLikeMax {
			stats.dateLikeMax = record.DateLike
		}
		if record.Field1Float32Valid {
			if !stats.field1Seen {
				stats.field1Min = record.Field1Float32
				stats.field1Max = record.Field1Float32
				stats.field1Seen = true
			}
			if record.Field1Float32 < stats.field1Min {
				stats.field1Min = record.Field1Float32
			}
			if record.Field1Float32 > stats.field1Max {
				stats.field1Max = record.Field1Float32
			}
		}
		if record.Field2Uint32 != 0 {
			stats.field2NonzeroCount++
		}
	}
	return stats
}

func (m DataPackageManifest) FindEntry(fileName string) (DataPackageEntry, bool) {
	for _, entry := range m.Entries {
		if strings.EqualFold(entry.FileName, fileName) {
			return entry, true
		}
	}
	return DataPackageEntry{}, false
}

func (m DataPackageManifest) EntriesByPrefix(prefix string) []DataPackageEntry {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil
	}
	out := make([]DataPackageEntry, 0)
	for _, entry := range m.Entries {
		if strings.HasPrefix(strings.ToLower(entry.FileName), prefix) {
			out = append(out, entry)
		}
	}
	return out
}

func (m DataPackageManifest) EntriesByExtension(ext string) []DataPackageEntry {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return nil
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	out := make([]DataPackageEntry, 0)
	for _, entry := range m.Entries {
		if strings.ToLower(filepath.Ext(entry.FileName)) == ext {
			out = append(out, entry)
		}
	}
	return out
}

func FilterDataPackageManifestByPrefix(manifest DataPackageManifest, prefix string) DataPackageManifest {
	filtered := DataPackageManifest{Source: manifest.Source}
	for _, entry := range manifest.EntriesByPrefix(prefix) {
		addDataPackageManifestEntry(&filtered, entry)
	}
	return filtered
}

func FilterDataPackageLocalIndexByPrefix(index DataPackageLocalIndex, prefix string) DataPackageLocalIndex {
	filtered := DataPackageLocalIndex{Source: index.Source}
	for _, entry := range index.EntriesByPrefix(prefix) {
		addDataPackageLocalIndexEntry(&filtered, entry)
	}
	return filtered
}

func ParseDataPackageLocalIndex(source string, data []byte) (DataPackageLocalIndex, error) {
	index := DataPackageLocalIndex{Source: source}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	lineNo := 0
	section := ""
	for scanner.Scan() {
		lineNo++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != "md5" {
			continue
		}
		entry, reason, ok := parseDataPackageChecksumLine(line, lineNo)
		if !ok {
			index.SkippedLines = append(index.SkippedLines, DataPackageSkippedLine{
				Line:   lineNo,
				Raw:    rawLine,
				Reason: reason,
			})
			continue
		}
		addDataPackageLocalIndexEntry(&index, entry)
	}
	if err := scanner.Err(); err != nil {
		return DataPackageLocalIndex{}, fmt.Errorf("scan data package local index: %w", err)
	}
	return index, nil
}

func (i DataPackageLocalIndex) EntriesByPrefix(prefix string) []DataPackageChecksumEntry {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil
	}
	out := make([]DataPackageChecksumEntry, 0)
	for _, entry := range i.Entries {
		if strings.HasPrefix(strings.ToLower(entry.FileName), prefix) {
			out = append(out, entry)
		}
	}
	return out
}

func parseDataPackageManifestLine(line string, lineNo int) (DataPackageEntry, string, bool) {
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		return DataPackageEntry{}, "expected 3 comma-separated fields", false
	}
	fileName := strings.TrimSpace(parts[0])
	md5 := strings.ToLower(strings.TrimSpace(parts[1]))
	sizeRaw := strings.TrimSpace(parts[2])
	if fileName == "" {
		return DataPackageEntry{}, "empty filename", false
	}
	if !isMD5Hex(md5) {
		return DataPackageEntry{}, "invalid md5", false
	}
	size, err := strconv.ParseInt(sizeRaw, 10, 64)
	if err != nil || size < 0 {
		return DataPackageEntry{}, "invalid size", false
	}
	return DataPackageEntry{
		FileName: fileName,
		MD5:      md5,
		Size:     size,
		Line:     lineNo,
	}, "", true
}

func parseDataPackageChecksumLine(line string, lineNo int) (DataPackageChecksumEntry, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return DataPackageChecksumEntry{}, "expected filename=md5", false
	}
	fileName := strings.TrimSpace(parts[0])
	md5 := strings.ToLower(strings.TrimSpace(parts[1]))
	if fileName == "" {
		return DataPackageChecksumEntry{}, "empty filename", false
	}
	if !isMD5Hex(md5) {
		return DataPackageChecksumEntry{}, "invalid md5", false
	}
	return DataPackageChecksumEntry{FileName: fileName, MD5: md5, Line: lineNo}, "", true
}

func addDataPackageManifestEntry(manifest *DataPackageManifest, entry DataPackageEntry) {
	manifest.Entries = append(manifest.Entries, entry)
	manifest.TotalSize += entry.Size
	manifest.PackageCount++
	switch strings.ToLower(filepath.Ext(entry.FileName)) {
	case ".local":
		manifest.LocalCount++
	case ".dat":
		manifest.DatCount++
	case ".zip":
		manifest.ZipCount++
	default:
		manifest.OtherCount++
	}
}

func addDataPackageLocalIndexEntry(index *DataPackageLocalIndex, entry DataPackageChecksumEntry) {
	index.Entries = append(index.Entries, entry)
	index.EntryCount++
	switch strings.ToLower(filepath.Ext(entry.FileName)) {
	case ".dat":
		index.DatCount++
	default:
		index.OtherCount++
	}
}

func isMD5Hex(raw string) bool {
	if len(raw) != 32 {
		return false
	}
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

func isHTMLChallengeResponse(contentType string, data []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	n := len(trimmed)
	if n > 64 {
		n = 64
	}
	prefix := strings.ToLower(string(trimmed[:n]))
	return strings.HasPrefix(prefix, "<script") ||
		strings.HasPrefix(prefix, "<html") ||
		strings.HasPrefix(prefix, "<!doctype")
}
