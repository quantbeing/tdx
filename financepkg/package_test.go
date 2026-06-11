package financepkg

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestParsePackageReadsZipDatRecordsAndPreservesRaw(t *testing.T) {
	dat, _, fieldRaw := testFinanceDAT(t)
	payload := testZipPayload(t, "gpcw20240630.dat", dat)

	got, err := ParsePackage(payload)
	if err != nil {
		t.Fatalf("ParsePackage: %v", err)
	}

	if got.ReportDate != 20240630 {
		t.Fatalf("ReportDate = %d, want 20240630", got.ReportDate)
	}
	if got.FieldCount != 2 {
		t.Fatalf("FieldCount = %d, want 2", got.FieldCount)
	}
	if !bytes.Equal(got.Raw, dat) {
		t.Fatal("Package.Raw does not preserve the dat payload")
	}
	if len(got.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(got.Records))
	}

	rec := got.Records[0]
	if rec.Code != "600519" || rec.Market != '1' {
		t.Fatalf("record identity = %+v", rec)
	}
	if len(rec.Fields) != 2 || rec.Fields[0] != 1.25 || rec.Fields[1] != -2.5 {
		t.Fatalf("Fields = %+v, want [1.25 -2.5]", rec.Fields)
	}
	if !bytes.Contains(rec.Raw, fieldRaw) {
		t.Fatalf("Record.Raw = %x, want it to contain field payload %x", rec.Raw, fieldRaw)
	}
}

func TestParsePackageReadsRawDatPayload(t *testing.T) {
	dat, _, _ := testFinanceDAT(t)

	got, err := ParsePackage(dat)
	if err != nil {
		t.Fatalf("ParsePackage(raw): %v", err)
	}

	if got.ReportDate != 20240630 || got.FieldCount != 2 || len(got.Records) != 1 {
		t.Fatalf("parsed package = %+v", got)
	}
	if got.Records[0].Code != "600519" || got.Records[0].Fields[1] != -2.5 {
		t.Fatalf("record = %+v", got.Records[0])
	}
}

func TestParsePackageReturnsMissingDATForZipWithoutDat(t *testing.T) {
	payload := testZipPayload(t, "readme.txt", []byte("not a dat file"))

	_, err := ParsePackage(payload)
	if !errors.Is(err, ErrMissingDAT) {
		t.Fatalf("err = %v, want ErrMissingDAT", err)
	}
	assertParseErrorText(t, err)
}

func TestParsePackageReturnsInvalidHeaderForShortHeader(t *testing.T) {
	_, err := ParsePackage([]byte{1, 2, 3})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("err = %v, want ErrInvalidHeader", err)
	}
	assertParseErrorText(t, err)
}

func TestParsePackageReturnsShortStockItem(t *testing.T) {
	dat := make([]byte, financePackageHeaderSize+financeStockItemSize-1)
	binary.LittleEndian.PutUint32(dat[2:6], 20240630)
	binary.LittleEndian.PutUint16(dat[6:8], 1)
	binary.LittleEndian.PutUint32(dat[12:16], 8)

	_, err := ParsePackage(dat)
	if !errors.Is(err, ErrShortStockItem) {
		t.Fatalf("err = %v, want ErrShortStockItem", err)
	}
	assertParseErrorText(t, err)
}

func TestParsePackageReturnsShortFieldPayload(t *testing.T) {
	dat, _, _ := testFinanceDAT(t)
	dat = dat[:len(dat)-1]

	_, err := ParsePackage(dat)
	if !errors.Is(err, ErrShortFieldPayload) {
		t.Fatalf("err = %v, want ErrShortFieldPayload", err)
	}
	assertParseErrorText(t, err)
}

func TestParsePackageRejectsHostileFieldOffsets(t *testing.T) {
	tests := []struct {
		name     string
		dat      []byte
		wantErr  error
		wantText string
	}{
		{
			name:     "field offset points into item table",
			dat:      testFinanceDATWithFieldOffsets(t, 4, nil, financePackageHeaderSize),
			wantErr:  ErrInvalidFieldOffset,
			wantText: "field offset",
		},
		{
			name: "field offset is reused",
			dat: testFinanceDATWithFieldOffsets(t, 4, []byte{
				0x00, 0x00, 0x80, 0x3f,
			}, financePackageHeaderSize+2*financeStockItemSize, financePackageHeaderSize+2*financeStockItemSize),
			wantErr:  ErrReusedFieldOffset,
			wantText: "reused field offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePackage(tt.dat)
			if err == nil {
				t.Fatal("ParsePackage succeeded, want hostile field offset error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("err = %v, want text containing %q", err, tt.wantText)
			}
			assertParseErrorText(t, err)
		})
	}
}

func TestParsePackageRejectsHugeDecodedAllocationHeader(t *testing.T) {
	const hostileReportSize = uint32(1 << 30)
	dat := testFinanceDATWithFieldOffsets(t, hostileReportSize, nil,
		financePackageHeaderSize+2*financeStockItemSize,
		financePackageHeaderSize+2*financeStockItemSize+hostileReportSize,
	)

	_, err := ParsePackage(dat)
	if err == nil {
		t.Fatal("ParsePackage succeeded, want decoded allocation size error")
	}
	if !errors.Is(err, ErrPackageTooLarge) {
		t.Fatalf("err = %v, want ErrPackageTooLarge", err)
	}
	for _, want := range []string{"too large", "decoded field bytes", "limit=", "actual="} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want text containing %q", err, want)
		}
	}
}

func TestParsePackageRejectsZipDATEntryOverLimit(t *testing.T) {
	payload := testZipPayloadWithUncompressedSize(t, "gpcw20240630.dat", []byte("small dat"), 1<<31)

	_, err := ParsePackage(payload)
	if err == nil {
		t.Fatal("ParsePackage succeeded, want zip DAT size error")
	}
	if !errors.Is(err, ErrPackageTooLarge) {
		t.Fatalf("err = %v, want ErrPackageTooLarge", err)
	}
	for _, want := range []string{"too large", "dat size", "limit=", "actual="} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want text containing %q", err, want)
		}
	}
}

func testFinanceDAT(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()

	var dat []byte
	dat = binary.LittleEndian.AppendUint16(dat, 1)
	dat = binary.LittleEndian.AppendUint32(dat, 20240630)
	dat = binary.LittleEndian.AppendUint16(dat, 1)
	dat = binary.LittleEndian.AppendUint32(dat, 0)
	dat = binary.LittleEndian.AppendUint32(dat, 8)
	dat = binary.LittleEndian.AppendUint32(dat, 0)

	item := make([]byte, financeStockItemSize)
	copy(item[0:6], []byte("600519"))
	item[6] = '1'
	fieldOffset := financePackageHeaderSize + financeStockItemSize
	binary.LittleEndian.PutUint32(item[7:11], uint32(fieldOffset))
	dat = append(dat, item...)

	fieldRaw := testAppendFloat32(nil, 1.25)
	fieldRaw = testAppendFloat32(fieldRaw, -2.5)
	dat = append(dat, fieldRaw...)

	return dat, item, fieldRaw
}

func testFinanceDATWithFieldOffsets(t *testing.T, reportSize uint32, sharedPayload []byte, offsets ...uint32) []byte {
	t.Helper()

	recordCount := len(offsets)
	tableSize := financePackageHeaderSize + recordCount*financeStockItemSize
	dat := make([]byte, tableSize)
	binary.LittleEndian.PutUint16(dat[0:2], 1)
	binary.LittleEndian.PutUint32(dat[2:6], 20240630)
	binary.LittleEndian.PutUint16(dat[6:8], uint16(recordCount))
	binary.LittleEndian.PutUint32(dat[12:16], reportSize)

	for i, fieldOffset := range offsets {
		item := dat[financePackageHeaderSize+i*financeStockItemSize : financePackageHeaderSize+(i+1)*financeStockItemSize]
		copy(item[0:6], []byte("600519"))
		item[6] = '1'
		binary.LittleEndian.PutUint32(item[7:11], fieldOffset)
	}

	if len(sharedPayload) > 0 {
		dat = append(dat, sharedPayload...)
	}
	return dat
}

func testZipPayload(t *testing.T, name string, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}
	return buf.Bytes()
}

func testZipPayloadWithUncompressedSize(t *testing.T, name string, data []byte, size uint32) []byte {
	t.Helper()

	payload := testZipPayload(t, name, data)
	centralDirectoryHeader := []byte{0x50, 0x4b, 0x01, 0x02}
	idx := bytes.Index(payload, centralDirectoryHeader)
	if idx < 0 {
		t.Fatal("central directory header not found")
	}
	binary.LittleEndian.PutUint32(payload[idx+24:idx+28], size)
	return payload
}

func testAppendFloat32(out []byte, v float32) []byte {
	return binary.LittleEndian.AppendUint32(out, math.Float32bits(v))
}

func assertParseErrorText(t *testing.T, err error) {
	t.Helper()

	text := err.Error()
	for _, want := range []string{"offset=", "expected=", "actual="} {
		if !strings.Contains(text, want) {
			t.Fatalf("error %q does not contain %q", text, want)
		}
	}
}
