package command

import (
	"encoding/binary"
	"testing"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

func BenchmarkParseSecurityList(b *testing.B) {
	body := buildBenchmarkSecurityListBody(1000)
	cmd := NewSecurityListCommand(model.MarketSH, 0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseSecurityBars(b *testing.B) {
	body := buildBenchmarkBarsBody(800, false)
	cmd := NewSecurityBarsCommand(model.MarketSH, "600519", model.KlineDay, 0, 800)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseIndexBars(b *testing.B) {
	body := buildBenchmarkBarsBody(800, true)
	cmd := NewIndexBarsCommand(model.MarketSH, "000001", model.KlineDay, 0, 800)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseSecurityQuotes(b *testing.B) {
	body := buildBenchmarkQuoteBody(80)
	symbols := make([]model.Symbol, 80)
	for i := range symbols {
		symbols[i] = model.Symbol{Market: model.MarketSH, Code: "600519"}
	}
	cmd := NewSecurityQuotesCommand(symbols)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMinuteTime(b *testing.B) {
	body := buildBenchmarkMinuteBody(240)
	cmd := NewMinuteTimeDataCommand(model.MarketSH, "600519")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseTransactions(b *testing.B) {
	body := buildBenchmarkTransactionBody(800)
	cmd := NewTransactionDataCommand(model.MarketSH, "600519", 0, 800)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cmd.ParseResponse(body); err != nil {
			b.Fatal(err)
		}
	}
}

func buildBenchmarkSecurityListBody(count int) []byte {
	body := make([]byte, 2, 2+count*securityListRecordSize)
	binary.LittleEndian.PutUint16(body, uint16(count))
	for i := 0; i < count; i++ {
		row := make([]byte, securityListRecordSize)
		copy(row[0:6], []byte("600519"))
		binary.LittleEndian.PutUint16(row[6:8], 100)
		copy(row[8:16], []byte{0xb9, 0xf3, 0xd6, 0xdd, 0xc3, 0xa9, 0xcc, 0xa8})
		row[20] = 2
		binary.LittleEndian.PutUint32(row[21:25], 0xb04a0282)
		body = append(body, row...)
	}
	return body
}

func buildBenchmarkBarsBody(count int, withBreadth bool) []byte {
	body := make([]byte, 0, 2+count*32)
	body = binary.LittleEndian.AppendUint16(body, uint16(count))
	for i := 0; i < count; i++ {
		body = binary.LittleEndian.AppendUint32(body, uint32(20260609))
		body = append(body, codec.PutPrice(10000)...)
		body = append(body, codec.PutPrice(500)...)
		body = append(body, codec.PutPrice(800)...)
		body = append(body, codec.PutPrice(-200)...)
		body = binary.LittleEndian.AppendUint32(body, 0xb04a0282)
		body = binary.LittleEndian.AppendUint32(body, 0xb04a0282)
		if withBreadth {
			body = binary.LittleEndian.AppendUint16(body, 123)
			body = binary.LittleEndian.AppendUint16(body, 456)
		}
	}
	return body
}

func buildBenchmarkQuoteBody(count int) []byte {
	body := []byte{0xb1, 0xcb}
	body = binary.LittleEndian.AppendUint16(body, uint16(count))
	for i := 0; i < count; i++ {
		body = append(body, byte(model.MarketSH))
		body = append(body, []byte("600519")...)
		body = binary.LittleEndian.AppendUint16(body, 11)
		for _, v := range []int{1050, -50, -20, 10, -80, 14999212, -1050, 10000, 100} {
			body = append(body, codec.PutPrice(v)...)
		}
		body = binary.LittleEndian.AppendUint32(body, 0x4ab00282)
		for _, v := range []int{400, 600, -1, 22694} {
			body = append(body, codec.PutPrice(v)...)
		}
		for level := 1; level <= 5; level++ {
			body = append(body, codec.PutPrice(-level)...)
			body = append(body, codec.PutPrice(level)...)
			body = append(body, codec.PutPrice(level*10)...)
			body = append(body, codec.PutPrice(level*12)...)
		}
		body = binary.LittleEndian.AppendUint16(body, 7)
		for _, v := range []int{1, 2, 3, 4} {
			body = append(body, codec.PutPrice(v)...)
		}
		body = binary.LittleEndian.AppendUint16(body, uint16(int16(25)))
		body = binary.LittleEndian.AppendUint16(body, 9)
	}
	return body
}

func buildBenchmarkMinuteBody(count int) []byte {
	body := make([]byte, 0, 2+count*8)
	body = binary.LittleEndian.AppendUint16(body, uint16(count))
	body = binary.LittleEndian.AppendUint16(body, 0)
	for i := 0; i < count; i++ {
		body = append(body, codec.PutPrice(1000+i)...)
		body = append(body, codec.PutPrice(i)...)
		body = append(body, codec.PutPrice(100+i)...)
	}
	return body
}

func buildBenchmarkTransactionBody(count int) []byte {
	body := make([]byte, 0, 2+count*12)
	body = binary.LittleEndian.AppendUint16(body, uint16(count))
	for i := 0; i < count; i++ {
		body = binary.LittleEndian.AppendUint16(body, uint16(9*60+30+i%240))
		for _, v := range []int{1050 + i, 200, 3, 1, 99} {
			body = append(body, codec.PutPrice(v)...)
		}
	}
	return body
}
