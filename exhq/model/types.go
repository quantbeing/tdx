package model

type MarketID uint8

type Server struct {
	Name string `json:"name,omitempty"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (s Server) Addr() string {
	return s.Host + ":" + itoa(s.Port)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

type KlineCategory uint16

const (
	KlineMinute1  KlineCategory = 7
	KlineMinute3  KlineCategory = 8
	KlineMinute5  KlineCategory = 0
	KlineMinute15 KlineCategory = 1
	KlineMinute30 KlineCategory = 2
	KlineMinute60 KlineCategory = 3
	KlineDay      KlineCategory = 4
	KlineWeek     KlineCategory = 5
	KlineMonth    KlineCategory = 6
	KlineYear     KlineCategory = 9
	KlineSeason   KlineCategory = 10
	KlineYearAlt  KlineCategory = 11
)

type Market struct {
	MarketID  MarketID
	Category  uint8
	Name      string
	ShortName string
	Unknown   []byte
	Raw       []byte
}

type Instrument struct {
	Category uint8
	Market   MarketID
	Code     string
	Name     string
	Desc     string
	Unknown1 [3]byte
	Unknown  []byte
	Raw      []byte
}

type QuoteLevel struct {
	Price  float64
	Volume uint32
}

type Quote struct {
	Market        MarketID
	Code          string
	PreClose      float64
	Open          float64
	High          float64
	Low           float64
	Price         float64
	OpenVolume    uint32
	OpenInterest  uint32
	TotalVolume   uint32
	CurrentVolume uint32
	InnerVolume   uint32
	OuterVolume   uint32
	Amount        float64
	Bid           [5]QuoteLevel
	Ask           [5]QuoteLevel
	Unknown1      uint32
	Unknown2      uint32
	Unknown3      uint32
	Unknown       []byte
	Raw           []byte
}

type Bar struct {
	Market          MarketID
	Code            string
	Category        KlineCategory
	Year            int
	Month           int
	Day             int
	Hour            int
	Minute          int
	Open            float64
	High            float64
	Low             float64
	Close           float64
	Position        uint32
	Trade           uint32
	Price           float64
	SettlementPrice float64
	Unknown         []byte
	Raw             []byte
}

type MinuteTime struct {
	Market       MarketID
	Code         string
	Date         int
	Hour         int
	Minute       int
	Price        float64
	AvgPrice     float64
	Volume       uint32
	OpenInterest uint32
	Unknown      []byte
	Raw          []byte
}

type Transaction struct {
	Market         MarketID
	Code           string
	Date           int
	Hour           int
	Minute         int
	Second         int
	Price          float64
	Volume         uint32
	PositionChange int32
	Nature         uint16
	NatureMark     int
	NatureValue    int
	Direction      int
	Unknown        []byte
	Raw            []byte
}

type PartialResult[T any] struct {
	Items    []T
	Failures []OperationError
}

type OperationError struct {
	Operation string
	Market    MarketID
	Start     int
	Count     int
	Server    Server
	Err       string
}
