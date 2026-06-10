package model

import "testing"

func TestKlineCategoryValuesMatchTDXWireCategories(t *testing.T) {
	cases := []struct {
		name string
		got  KlineCategory
		want KlineCategory
	}{
		{name: "minute 1", got: KlineMinute1, want: 7},
		{name: "minute 3", got: KlineMinute3, want: 8},
		{name: "minute 5", got: KlineMinute5, want: 0},
		{name: "minute 15", got: KlineMinute15, want: 1},
		{name: "minute 30", got: KlineMinute30, want: 2},
		{name: "minute 60", got: KlineMinute60, want: 3},
		{name: "day", got: KlineDay, want: 4},
		{name: "week", got: KlineWeek, want: 5},
		{name: "month", got: KlineMonth, want: 6},
		{name: "season", got: KlineSeason, want: 10},
		{name: "year", got: KlineYear, want: 9},
		{name: "year alt", got: KlineYearAlt, want: 11},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s category = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}
