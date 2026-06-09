package codec

import (
	"encoding/binary"
	"fmt"
)

type TDXTime struct {
	Year   int
	Month  int
	Day    int
	Hour   int
	Minute int
}

func GetDateTime(category int, data []byte, pos int) (TDXTime, int, error) {
	if pos+4 > len(data) {
		return TDXTime{}, pos, fmt.Errorf("tdx datetime truncated at offset %d", pos)
	}
	if category < 4 || category == 7 || category == 8 {
		zipday := binary.LittleEndian.Uint16(data[pos : pos+2])
		tminutes := binary.LittleEndian.Uint16(data[pos+2 : pos+4])
		return TDXTime{
			Year:   int(zipday>>11) + 2004,
			Month:  int((zipday % 2048) / 100),
			Day:    int((zipday % 2048) % 100),
			Hour:   int(tminutes / 60),
			Minute: int(tminutes % 60),
		}, pos + 4, nil
	}
	zipday := binary.LittleEndian.Uint32(data[pos : pos+4])
	return TDXTime{
		Year:   int(zipday / 10000),
		Month:  int((zipday % 10000) / 100),
		Day:    int(zipday % 100),
		Hour:   15,
		Minute: 0,
	}, pos + 4, nil
}

func GetTime(data []byte, pos int) (hour int, minute int, newPos int, err error) {
	if pos+2 > len(data) {
		return 0, 0, pos, fmt.Errorf("tdx time truncated at offset %d", pos)
	}
	tminutes := binary.LittleEndian.Uint16(data[pos : pos+2])
	return int(tminutes / 60), int(tminutes % 60), pos + 2, nil
}
