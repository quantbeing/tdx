package model

type Market uint16

const (
	MarketSZ Market = 0
	MarketSH Market = 1
	MarketBJ Market = 2
)

func (m Market) String() string {
	switch m {
	case MarketSZ:
		return "SZ"
	case MarketSH:
		return "SH"
	case MarketBJ:
		return "BJ"
	default:
		return "UNKNOWN"
	}
}

type KlineCategory uint16

const (
	KlineMinute5  KlineCategory = 0
	KlineMinute15 KlineCategory = 1
	KlineMinute30 KlineCategory = 2
	KlineMinute60 KlineCategory = 3
	KlineDay      KlineCategory = 4
	KlineWeek     KlineCategory = 5
	KlineMonth    KlineCategory = 6
	KlineMinute1  KlineCategory = 7
	KlineMinute3  KlineCategory = 8
	KlineYear     KlineCategory = 9
	KlineSeason   KlineCategory = 10
	KlineYearAlt  KlineCategory = 11
)
