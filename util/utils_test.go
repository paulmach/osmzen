package util

import (
	"math"
	"testing"
)

func TestToFloat64(t *testing.T) {
	cases := []struct {
		name   string
		num    string
		result float64
		ok     bool
	}{
		{"int", "123", 123, true},
		{"negative", "-123", -123, true},
		{"float", "-1.5", -1.5, true},
		{"with spaces", "  -1.5   ", -1.5, true},
		{"with leters", "abcd", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := ToFloat64(tc.num)
			if ok != tc.ok {
				t.Fatalf("not parsed correctly: %v != %v", ok, tc.ok)
			}

			if v != tc.result {
				t.Errorf("result not correct: %v != %v", v, tc.result)
			}
		})
	}
}

func TestToFloat64Meteres(t *testing.T) {
	cases := []struct {
		name   string
		num    string
		result float64
		ok     bool
	}{
		{"int", "123", 123, true},
		{"negative", "-123", -123, true},
		{"float", "-1.5", -1.5, true},
		{"with spaces", "  -1.5   ", -1.5, true},
		{"with leters", "abcd", 0, false},

		// imperial
		{"foot", `1'`, 12 * metersPerInch, true},
		{"foot inch", `1'5"`, 17 * metersPerInch, true},
		{"foot space inch", `1' 6"`, 18 * metersPerInch, true},

		// units
		{"mile", `1mi`, 1 * unitFactors["mi"], true},
		{"mile space", `2 mi`, 2 * unitFactors["mi"], true},
		{"kilometer", `3km`, 3 * unitFactors["km"], true},
		{"meter", `4m`, 4 * unitFactors["m"], true},
		{"meter space", `4 m`, 4 * unitFactors["m"], true},
		{"nautical mile", `5nmi`, 5 * unitFactors["nmi"], true},
		{"foot", `6 ft`, 6 * unitFactors["ft"], true},

		{"letters", `abcd`, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := ToFloat64Meters(tc.num)
			if ok != tc.ok {
				t.Fatalf("not parsed correctly: %v != %v", ok, tc.ok)
			}

			if math.Abs(v-tc.result) > 0.0001 {
				t.Errorf("result not correct: %v != %v", v, tc.result)
			}
		})
	}
}

func TestBuildingHeight(t *testing.T) {
	cases := []struct {
		name   string
		height string
		levels string
		result float64
		ok     bool
	}{
		{"height", "123", "", 123, true},
		{"levels", "", "7", 23, true},
		{"weird values", "asdf", "ghjk", 1.0e10, true},
		{"none", "", "", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := BuildingHeight(tc.height, tc.levels)
			if ok != tc.ok {
				t.Fatalf("not parsed correctly: %v != %v", ok, tc.ok)
			}

			if math.Abs(v-tc.result) > 0.0001 {
				t.Errorf("result not correct: %v != %v", v, tc.result)
			}
		})
	}
}
