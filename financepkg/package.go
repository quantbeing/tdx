package financepkg

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	financePackageHeaderSize = 20
	financeStockItemSize     = 11

	// MaxPackageDATSize caps compressed-package .dat payloads before decoding.
	MaxPackageDATSize uint64 = 64 << 20
	// MaxDecodedFieldBytes caps the sum of per-record decoded field payloads.
	MaxDecodedFieldBytes uint64 = MaxPackageDATSize
)

var (
	ErrMissingDAT         = errors.New("missing .dat")
	ErrInvalidHeader      = errors.New("invalid header")
	ErrShortStockItem     = errors.New("short stock item")
	ErrShortFieldPayload  = errors.New("short field payload")
	ErrPackageTooLarge    = errors.New("package too large")
	ErrInvalidFieldOffset = errors.New("invalid field offset")
	ErrReusedFieldOffset  = errors.New("reused field offset")
)

type ParseError struct {
	Kind     error
	Offset   int
	Expected int
	Actual   int
}

func (e *ParseError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v: offset=%d expected=%d actual=%d", e.Kind, e.Offset, e.Expected, e.Actual)
}

func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

type SizeError struct {
	Kind   error
	Scope  string
	Limit  uint64
	Actual uint64
}

func (e *SizeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Scope == "" {
		return fmt.Sprintf("%v: limit=%d actual=%d", e.Kind, e.Limit, e.Actual)
	}
	return fmt.Sprintf("%v: %s limit=%d actual=%d", e.Kind, e.Scope, e.Limit, e.Actual)
}

func (e *SizeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

type Package struct {
	ReportDate int
	FieldCount int
	Records    []Record
	Raw        []byte
}

type Record struct {
	Code   string
	Market byte
	Fields []float32
	Raw    []byte
}

func ParsePackage(data []byte) (*Package, error) {
	raw, err := datPayload(data)
	if err != nil {
		return nil, err
	}
	if uint64(len(raw)) > MaxPackageDATSize {
		return nil, sizeErr("dat size", MaxPackageDATSize, uint64(len(raw)))
	}
	if len(raw) < financePackageHeaderSize {
		return nil, parseErr(ErrInvalidHeader, 0, financePackageHeaderSize, len(raw))
	}

	reportDate := int(binary.LittleEndian.Uint32(raw[2:6]))
	recordCount64 := uint64(binary.LittleEndian.Uint16(raw[6:8]))
	reportSize64 := uint64(binary.LittleEndian.Uint32(raw[12:16]))
	if reportSize64%4 != 0 {
		return nil, parseErr(ErrInvalidHeader, 12, 4, int(reportSize64%4))
	}
	rawSize := uint64(len(raw))
	tableSize := uint64(financePackageHeaderSize) + recordCount64*uint64(financeStockItemSize)
	if tableSize > rawSize {
		return nil, parseErr(
			ErrShortStockItem,
			financePackageHeaderSize,
			int(tableSize)-financePackageHeaderSize,
			remaining(raw, financePackageHeaderSize),
		)
	}
	decodedFieldBytes := recordCount64 * reportSize64
	if decodedFieldBytes > MaxDecodedFieldBytes {
		return nil, sizeErr("decoded field bytes", MaxDecodedFieldBytes, decodedFieldBytes)
	}

	recordCount := int(recordCount64)
	reportSize := int(reportSize64)
	fieldOffsets := make([]int, recordCount)
	seenFieldOffsets := make(map[uint64]struct{}, recordCount)
	for i := 0; i < recordCount; i++ {
		itemOffset := financePackageHeaderSize + i*financeStockItemSize
		item := raw[itemOffset : itemOffset+financeStockItemSize]
		fieldOffset := uint64(binary.LittleEndian.Uint32(item[7:11]))
		if fieldOffset > rawSize {
			return nil, parseErr(ErrShortFieldPayload, int(fieldOffset), reportSize, 0)
		}
		if reportSize64 > 0 {
			if fieldOffset < tableSize {
				return nil, parseErr(ErrInvalidFieldOffset, int(fieldOffset), int(tableSize), int(fieldOffset))
			}
			if reportSize64 > rawSize-fieldOffset {
				return nil, parseErr(ErrShortFieldPayload, int(fieldOffset), reportSize, remaining(raw, int(fieldOffset)))
			}
			if _, ok := seenFieldOffsets[fieldOffset]; ok {
				return nil, parseErr(ErrReusedFieldOffset, int(fieldOffset), reportSize, int(fieldOffset))
			}
			seenFieldOffsets[fieldOffset] = struct{}{}
		}
		fieldOffsets[i] = int(fieldOffset)
	}
	fieldCount := reportSize / 4

	pkg := &Package{
		ReportDate: reportDate,
		FieldCount: fieldCount,
		Records:    make([]Record, 0, recordCount),
		Raw:        append([]byte(nil), raw...),
	}

	for i := 0; i < recordCount; i++ {
		itemOffset := financePackageHeaderSize + i*financeStockItemSize
		item := raw[itemOffset : itemOffset+financeStockItemSize]
		fieldOffset := fieldOffsets[i]
		fieldRaw := raw[fieldOffset : fieldOffset+reportSize]
		fields := make([]float32, fieldCount)
		for j := range fields {
			start := j * 4
			fields[j] = math.Float32frombits(binary.LittleEndian.Uint32(fieldRaw[start : start+4]))
		}

		recordRaw := make([]byte, 0, len(item)+len(fieldRaw))
		recordRaw = append(recordRaw, item...)
		recordRaw = append(recordRaw, fieldRaw...)
		pkg.Records = append(pkg.Records, Record{
			Code:   strings.TrimRight(string(item[0:6]), "\x00 "),
			Market: item[6],
			Fields: fields,
			Raw:    recordRaw,
		})
	}

	return pkg, nil
}

func datPayload(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return data, nil
	}

	for _, file := range zr.File {
		if !strings.HasSuffix(strings.ToLower(file.Name), ".dat") {
			continue
		}
		if file.UncompressedSize64 > MaxPackageDATSize {
			return nil, sizeErr("dat size", MaxPackageDATSize, file.UncompressedSize64)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		limited := &io.LimitedReader{R: rc, N: int64(MaxPackageDATSize) + 1}
		raw, readErr := io.ReadAll(limited)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if uint64(len(raw)) > MaxPackageDATSize {
			return nil, sizeErr("dat size", MaxPackageDATSize, uint64(len(raw)))
		}
		return raw, nil
	}

	return nil, parseErr(ErrMissingDAT, 0, 1, 0)
}

func parseErr(kind error, offset int, expected int, actual int) error {
	return &ParseError{Kind: kind, Offset: offset, Expected: expected, Actual: actual}
}

func sizeErr(scope string, limit uint64, actual uint64) error {
	return &SizeError{Kind: ErrPackageTooLarge, Scope: scope, Limit: limit, Actual: actual}
}

func remaining(data []byte, offset int) int {
	if offset >= len(data) {
		return 0
	}
	return len(data) - offset
}
