package model

import "testing"

func TestKlineCategoryValuesMatchTDXWireCategories(t *testing.T) {
	cases := map[string]struct {
		got  KlineCategory
		want KlineCategory
	}{
		"minute 5":  {got: KlineMinute5, want: 0},
		"minute 15": {got: KlineMinute15, want: 1},
		"minute 30": {got: KlineMinute30, want: 2},
		"minute 60": {got: KlineMinute60, want: 3},
		"day":       {got: KlineDay, want: 4},
		"week":      {got: KlineWeek, want: 5},
		"month":     {got: KlineMonth, want: 6},
		"minute 1":  {got: KlineMinute1, want: 7},
		"minute 3":  {got: KlineMinute3, want: 8},
		"year":      {got: KlineYear, want: 9},
		"season":    {got: KlineSeason, want: 10},
		"year alt":  {got: KlineYearAlt, want: 11},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s category = %d, want %d", name, tc.got, tc.want)
		}
	}
}
