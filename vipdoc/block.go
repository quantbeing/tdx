package vipdoc

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantbeing/tdx/codec"
)

const (
	blockHeaderOffset = 384
	blockCountSize    = 2
	blockNameSize     = 9
	blockCodeSize     = 7
	blockCodeAreaSize = 2800
	blockRecordSize   = blockNameSize + 2 + 2 + blockCodeAreaSize
	maxBlockCodes     = blockCodeAreaSize / blockCodeSize
)

type BlockMember struct {
	Filename   string
	BlockIndex int
	BlockName  string
	BlockType  uint16
	StockCount uint16
	CodeIndex  int
	Code       string
	RawCode    []byte
	RawBlock   []byte
}

func (r *Reader) BlockFile(ctx context.Context, filename string) ([]BlockMember, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	path, err := r.blockPath(filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < blockHeaderOffset {
		return nil, newParseError(path, 0, blockHeaderOffset, len(data), ErrTruncatedFile)
	}
	if len(data) < blockHeaderOffset+blockCountSize {
		return nil, newParseError(path, blockHeaderOffset, blockCountSize, len(data)-blockHeaderOffset, ErrTruncatedFile)
	}

	count := int(binary.LittleEndian.Uint16(data[blockHeaderOffset : blockHeaderOffset+blockCountSize]))
	pos := blockHeaderOffset + blockCountSize
	out := make([]BlockMember, 0)
	for i := 0; i < count; i++ {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		if pos+blockRecordSize > len(data) {
			return nil, newParseError(path, int64(pos), blockRecordSize, len(data)-pos, ErrTruncatedFile)
		}
		rawBlock := append([]byte(nil), data[pos:pos+blockRecordSize]...)
		name := decodeTDXString(rawBlock[0:blockNameSize])
		stockCount := binary.LittleEndian.Uint16(rawBlock[9:11])
		blockType := binary.LittleEndian.Uint16(rawBlock[11:13])
		if stockCount > maxBlockCodes {
			return nil, &UnsupportedError{
				Path:    path,
				Details: fmt.Sprintf("block_index=%d stock_count=%d max=%d exceeds supported block code area", i, stockCount, maxBlockCodes),
				Err:     ErrUnsupportedFormat,
			}
		}
		for j := 0; j < int(stockCount); j++ {
			start := 13 + j*blockCodeSize
			rawCode := append([]byte(nil), rawBlock[start:start+blockCodeSize]...)
			code := strings.TrimRight(string(rawCode), "\x00 ")
			if code == "" {
				continue
			}
			out = append(out, BlockMember{
				Filename:   filepath.Base(filename),
				BlockIndex: i,
				BlockName:  name,
				BlockType:  blockType,
				StockCount: stockCount,
				CodeIndex:  j,
				Code:       code,
				RawCode:    rawCode,
				RawBlock:   rawBlock,
			})
		}
		pos += blockRecordSize
	}
	return out, nil
}

func decodeTDXString(raw []byte) string {
	if idx := strings.IndexByte(string(raw), 0); idx >= 0 {
		raw = raw[:idx]
	}
	return codec.DecodeGBKBestEffort(raw)
}
