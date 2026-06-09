package codec

import (
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func DecodeGBKBestEffort(raw []byte) string {
	trimmed := strings.TrimRight(string(raw), "\x00")
	out, err := simplifiedchinese.GBK.NewDecoder().String(trimmed)
	if err != nil {
		return strings.ToValidUTF8(trimmed, "\uFFFD")
	}
	return strings.TrimSpace(out)
}
