package validation

import (
	"fmt"
	"testing"

	"github.com/quantbeing/tdx/model"
)

func BenchmarkValidateSecurities(b *testing.B) {
	items := make([]model.Security, 1000)
	for i := range items {
		items[i] = model.Security{Market: model.MarketSH, Code: "600519", Name: "贵州茅台", DecimalPoint: 2, Raw: make([]byte, 29)}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if result := ValidateSecurities("security_list", model.MarketSH, items); !result.OK {
			b.Fatalf("result = %+v", result)
		}
	}
}

func BenchmarkValidateSecurityUniverse(b *testing.B) {
	items := make([]model.Security, 5000)
	for i := range items {
		items[i] = model.Security{Market: model.MarketSH, Code: fmt.Sprintf("%06d", i), Name: "SEC", DecimalPoint: 2, Raw: make([]byte, 29)}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if result := ValidateSecurityUniverse("security_list_SH_full", model.MarketSH, len(items), items); !result.OK {
			b.Fatalf("result = %+v", result)
		}
	}
}

func BenchmarkValidateQuotes(b *testing.B) {
	symbols := make([]model.Symbol, 80)
	quotes := make([]model.Quote, 80)
	for i := range symbols {
		symbols[i] = model.Symbol{Market: model.MarketSH, Code: "600519"}
		quotes[i] = model.Quote{Market: model.MarketSH, Code: "600519", Price: model.NewPriceFromMilli(10500), Vol: 1, Amount: 1, Raw: []byte{1}}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if result := ValidateQuotes("security_quotes", symbols, quotes); !result.OK {
			b.Fatalf("result = %+v", result)
		}
	}
}
