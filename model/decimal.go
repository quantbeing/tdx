package model

import (
	"fmt"
	"strconv"
	"strings"
)

type Decimal struct {
	Mantissa int64
	Scale    int32
}

func NewDecimal(mantissa int64, scale int32) Decimal {
	return Decimal{Mantissa: mantissa, Scale: scale}
}

func NewPriceFromMilli(v int) Decimal {
	return Decimal{Mantissa: int64(v), Scale: 3}
}

func NewDecimalFromFloat(v float64, scale int32) Decimal {
	mul := int64(1)
	for i := int32(0); i < scale; i++ {
		mul *= 10
	}
	return Decimal{Mantissa: int64(v * float64(mul)), Scale: scale}
}

func (d Decimal) Float64() float64 {
	div := float64(1)
	for i := int32(0); i < d.Scale; i++ {
		div *= 10
	}
	return float64(d.Mantissa) / div
}

func (d Decimal) String() string {
	if d.Scale <= 0 {
		return strconv.FormatInt(d.Mantissa, 10)
	}
	negative := d.Mantissa < 0
	m := d.Mantissa
	if negative {
		m = -m
	}
	div := int64(1)
	for i := int32(0); i < d.Scale; i++ {
		div *= 10
	}
	whole := m / div
	frac := m % div
	out := fmt.Sprintf("%d.%0*d", whole, d.Scale, frac)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	if negative {
		return "-" + out
	}
	return out
}
