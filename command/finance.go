package command

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

const financeInfoSize = 4 + 2 + 2 + 4 + 4 + 30*4

type FinanceInfoCommand struct {
	Market model.Market
	Code   string
}

func NewFinanceInfoCommand(market model.Market, code string) FinanceInfoCommand {
	return FinanceInfoCommand{Market: market, Code: code}
}

func (c FinanceInfoCommand) Operation() string { return "finance_info" }

func (c FinanceInfoCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	req := mustHex("0c1f187600010b000b0010000100")
	req = append(req, byte(c.Market))
	req = append(req, code[:]...)
	return req, nil
}

func (c FinanceInfoCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 2+7+financeInfoSize {
		return nil, fmt.Errorf("finance_info response truncated: %d", len(body))
	}
	pos := 2
	market := model.Market(body[pos])
	code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00 ")
	pos += 7
	raw := append([]byte(nil), body[pos:pos+financeInfoSize]...)
	p := pos
	readF := func() float64 {
		v := math.Float32frombits(binary.LittleEndian.Uint32(body[p : p+4]))
		p += 4
		return float64(v)
	}
	liutongGuben := readF()
	province := binary.LittleEndian.Uint16(body[p : p+2])
	p += 2
	industry := binary.LittleEndian.Uint16(body[p : p+2])
	p += 2
	updatedDate := binary.LittleEndian.Uint32(body[p : p+4])
	p += 4
	ipoDate := binary.LittleEndian.Uint32(body[p : p+4])
	p += 4
	fields := make([]float64, 30)
	for i := range fields {
		fields[i] = readF()
	}
	scale := func(v float64) float64 { return v * 10000 }
	return model.FinanceInfo{
		Market: market, Code: code, LiutongGuben: scale(liutongGuben), Province: province, Industry: industry,
		UpdatedDate: updatedDate, IPODate: ipoDate,
		ZongGuben: scale(fields[0]), GuojiaGu: scale(fields[1]), FaqirenFarenGu: scale(fields[2]),
		FarenGu: scale(fields[3]), BGu: scale(fields[4]), HGu: scale(fields[5]), ZhigongGu: scale(fields[6]),
		ZongZichan: scale(fields[7]), LiudongZichan: scale(fields[8]), GudingZichan: scale(fields[9]),
		WuxingZichan: scale(fields[10]), GudongRenshu: fields[11], LiudongFuzhai: scale(fields[12]),
		ChangqiFuzhai: scale(fields[13]), ZibenGongjijin: scale(fields[14]), JingZichan: scale(fields[15]),
		ZhuyingShouru: scale(fields[16]), ZhuyingLirun: scale(fields[17]), YingshouZhangkuan: scale(fields[18]),
		YingyeLirun: scale(fields[19]), TouziShouyu: scale(fields[20]), JingyingXianjinliu: scale(fields[21]),
		ZongXianjinliu: scale(fields[22]), Cunhuo: scale(fields[23]), LirunZonghe: scale(fields[24]),
		ShuihouLirun: scale(fields[25]), JingLirun: scale(fields[26]), WeifenLirun: scale(fields[27]),
		MeigujingZichan: fields[28], Reserve2: fields[29], Raw: raw,
	}, nil
}

type XdxrInfoCommand struct {
	Market model.Market
	Code   string
}

func NewXdxrInfoCommand(market model.Market, code string) XdxrInfoCommand {
	return XdxrInfoCommand{Market: market, Code: code}
}

func (c XdxrInfoCommand) Operation() string { return "xdxr_info" }

func (c XdxrInfoCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	req := mustHex("0c1f187600010b000b000f000100")
	req = append(req, byte(c.Market))
	req = append(req, code[:]...)
	return req, nil
}

func (c XdxrInfoCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 11 {
		return []model.XdxrRecord{}, nil
	}
	pos := 9
	count := int(binary.LittleEndian.Uint16(body[pos : pos+2]))
	pos += 2
	out := make([]model.XdxrRecord, 0, count)
	for i := 0; i < count; i++ {
		start := pos
		if pos+7+1+4+1+16 > len(body) {
			return nil, fmt.Errorf("xdxr_info record %d truncated at offset %d", i, pos)
		}
		market := model.Market(body[pos])
		code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00 ")
		pos += 7
		pos++
		tm, next, err := codec.GetDateTime(int(model.KlineDay), body, pos)
		if err != nil {
			return nil, fmt.Errorf("xdxr_info datetime[%d]: %w", i, err)
		}
		pos = next
		category := body[pos]
		pos++
		chunk := body[pos : pos+16]
		pos += 16
		rec := model.XdxrRecord{
			Market: market, Code: code, Year: tm.Year, Month: tm.Month, Day: tm.Day,
			Category: category, Name: xdxrCategoryName(category), Raw: append([]byte(nil), body[start:pos]...),
		}
		switch category {
		case 1:
			fenhong := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[0:4]))) / 10
			peigujia := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[4:8])))
			songzhuangu := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12]))) / 10
			peigu := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[12:16]))) / 10
			rec.Fenhong, rec.Peigujia, rec.Songzhuangu, rec.Peigu = &fenhong, &peigujia, &songzhuangu, &peigu
		case 11, 12:
			suogu := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12])))
			rec.Suogu = &suogu
		case 13, 14:
			xingquanjia := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[0:4])))
			fenshu := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12])))
			rec.Xingquanjia, rec.Fenshu = &xingquanjia, &fenshu
		default:
			panqian := codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[0:4]))
			qianTotal := codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[4:8]))
			panhou := codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[8:12]))
			houTotal := codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[12:16]))
			rec.PanqianLiutong, rec.QianZongguben, rec.PanhouLiutong, rec.HouZongguben = &panqian, &qianTotal, &panhou, &houTotal
		}
		out = append(out, rec)
	}
	return out, nil
}

func xdxrCategoryName(category uint8) string {
	switch category {
	case 1:
		return "除权除息"
	case 2:
		return "送配股上市"
	case 3:
		return "非流通股上市"
	case 4:
		return "未知股本变动"
	case 5:
		return "股本变化"
	case 6:
		return "增发新股"
	case 7:
		return "股份回购"
	case 8:
		return "增发新股上市"
	case 9:
		return "转配股上市"
	case 10:
		return "可转债上市"
	case 11:
		return "扩缩股"
	case 12:
		return "非流通股缩股"
	case 13:
		return "送认购权证"
	case 14:
		return "送认沽权证"
	default:
		return fmt.Sprintf("%d", category)
	}
}
