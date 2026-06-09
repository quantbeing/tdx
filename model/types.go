package model

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

type ServerStat struct {
	Server    Server  `json:"server"`
	Successes uint64  `json:"successes"`
	Failures  uint64  `json:"failures"`
	Score     float64 `json:"score"`
	LastError string  `json:"last_error,omitempty"`
	LastOp    string  `json:"last_op,omitempty"`
	Cooling   bool    `json:"cooling"`
}

type Symbol struct {
	Market Market
	Code   string
}

type Security struct {
	Market       Market
	Code         string
	Name         string
	VolUnit      uint16
	DecimalPoint uint8
	PreClose     float64
	Unknown1     [4]byte
	Unknown2     [4]byte
	Raw          []byte
}

type Bar struct {
	Market    Market
	Code      string
	Category  KlineCategory
	Open      Decimal
	Close     Decimal
	High      Decimal
	Low       Decimal
	Vol       float64
	Amount    float64
	Year      int
	Month     int
	Day       int
	Hour      int
	Minute    int
	UpCount   uint16
	DownCount uint16
	Raw       []byte
}

type Quote struct {
	Market     Market
	Code       string
	Price      Decimal
	PreClose   Decimal
	Open       Decimal
	High       Decimal
	Low        Decimal
	Vol        float64
	CurVol     float64
	Amount     float64
	SVol       float64
	BVol       float64
	Active1    uint16
	Active2    uint16
	Bid        [5]QuoteLevel
	Ask        [5]QuoteLevel
	RiseSpeed  Decimal
	LimitUp    *Decimal
	LimitDown  *Decimal
	Unknown0   int
	Unknown1   int
	Unknown2   int
	Unknown3   int
	Unknown4   uint16
	Unknown5   int
	Unknown6   int
	Unknown7   int
	Unknown8   int
	ServerTime string
	Raw        []byte
}

type QuoteLevel struct {
	Price  Decimal
	Volume float64
}

type MinuteTime struct {
	Hour     int
	Minute   int
	Price    Decimal
	Volume   float64
	Unknown1 int
	Raw      []byte
}

type Transaction struct {
	Hour        int
	Minute      int
	Price       Decimal
	Vol         int
	NumOrders   int
	BuyOrSell   int
	UnknownLast int
	Raw         []byte
}

type MarketStat struct {
	UpCount        int     `json:"up_count"`
	DownCount      int     `json:"down_count"`
	NeutralCount   int     `json:"neutral_count"`
	SuspendedCount int     `json:"suspended_count"`
	TotalCount     int     `json:"total_count"`
	TotalAmount    float64 `json:"total_amount"`
	TotalVolume    float64 `json:"total_volume"`
}

type FundFlow struct {
	SuperIn   float64 `json:"super_in"`
	LargeIn   float64 `json:"large_in"`
	MediumIn  float64 `json:"medium_in"`
	SmallIn   float64 `json:"small_in"`
	SuperOut  float64 `json:"super_out"`
	LargeOut  float64 `json:"large_out"`
	MediumOut float64 `json:"medium_out"`
	SmallOut  float64 `json:"small_out"`
}

func (f FundFlow) MainNetInflow() float64 {
	return (f.SuperIn + f.LargeIn) - (f.SuperOut + f.LargeOut)
}

func (f FundFlow) TotalNetInflow() float64 {
	return (f.SuperIn + f.LargeIn + f.MediumIn + f.SmallIn) - (f.SuperOut + f.LargeOut + f.MediumOut + f.SmallOut)
}

type HistoricalFundFlow struct {
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	Day       int     `json:"day"`
	SuperIn   float64 `json:"super_in"`
	SuperOut  float64 `json:"super_out"`
	LargeIn   float64 `json:"large_in"`
	LargeOut  float64 `json:"large_out"`
	MediumIn  float64 `json:"medium_in"`
	MediumOut float64 `json:"medium_out"`
	SmallIn   float64 `json:"small_in"`
	SmallOut  float64 `json:"small_out"`
	Raw       []byte  `json:"raw"`
}

func (f HistoricalFundFlow) MainNetInflow() float64 {
	return (f.SuperIn + f.LargeIn) - (f.SuperOut + f.LargeOut)
}

type FinanceInfo struct {
	Market             Market
	Code               string
	LiutongGuben       float64
	ZongGuben          float64
	GuojiaGu           float64
	FaqirenFarenGu     float64
	FarenGu            float64
	BGu                float64
	HGu                float64
	ZhigongGu          float64
	Province           uint16
	Industry           uint16
	UpdatedDate        uint32
	IPODate            uint32
	GudongRenshu       float64
	ZongZichan         float64
	LiudongZichan      float64
	GudingZichan       float64
	WuxingZichan       float64
	LiudongFuzhai      float64
	ChangqiFuzhai      float64
	ZibenGongjijin     float64
	JingZichan         float64
	ZhuyingShouru      float64
	ZhuyingLirun       float64
	YingshouZhangkuan  float64
	YingyeLirun        float64
	TouziShouyu        float64
	JingyingXianjinliu float64
	ZongXianjinliu     float64
	Cunhuo             float64
	LirunZonghe        float64
	ShuihouLirun       float64
	JingLirun          float64
	WeifenLirun        float64
	MeigujingZichan    float64
	Reserve2           float64
	Raw                []byte
}

type XdxrRecord struct {
	Market         Market
	Code           string
	Year           int
	Month          int
	Day            int
	Category       uint8
	Name           string
	Fenhong        *float64
	Peigujia       *float64
	Songzhuangu    *float64
	Peigu          *float64
	Suogu          *float64
	Xingquanjia    *float64
	Fenshu         *float64
	PanqianLiutong *float64
	PanhouLiutong  *float64
	QianZongguben  *float64
	HouZongguben   *float64
	Raw            []byte
}

type CompanyInfoCategory struct {
	Name     string
	Filename string
	Start    int
	Length   int
	Raw      []byte
}

type Board struct {
	Name     string
	Category int
	Count    uint16
	Codes    []string
	Raw      []byte
}

type FileMeta struct {
	Filename string
	Size     int
	Hash     string
}

type PartialResult[T any] struct {
	Items    []T
	Failures []OperationError
}

type OperationError struct {
	Operation string
	Market    Market
	Start     int
	Count     int
	Server    Server
	Err       string
}
