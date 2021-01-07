package util

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

var (
	feetPattern   = regexp.MustCompile(`([+-]?[0-9.]+)\'(?: *([+-]?[0-9.]+)")?`)
	numberPattern = regexp.MustCompile(`([+-]?[0-9.]+)`)
	unitPattern   = regexp.MustCompile(`([+-]?[0-9.]+) *(mi|km|m|nmi|ft)`)
)

//  multiplicative conversion factor from the unit into meters.
// PLEASE: keep this in sync with the unit_pattern above.
var unitFactors = map[string]float64{
	"mi":  1609.3440,
	"km":  1000.0000,
	"m":   1.0000,
	"nmi": 1852.0000,
	"ft":  0.3048,
}

// ToFloat64 converts a string to a float64, if not a number returns
// false as the second argument.
func ToFloat64(x string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
	if err != nil {
		return 0, false
	}

	return f, true
}

const metersPerInch = 0.0254

// ToFloat64Meters converts the string in `1' 2"` or `1.5mi`
// into the equivalent meters.
func ToFloat64Meters(x string) (float64, bool) {
	x = strings.TrimSpace(x)
	if x == "" {
		return 0, false
	}

	if f, ok := ToFloat64(x); ok {
		return f, true
	}

	// try looking for a unit
	matches := unitPattern.FindStringSubmatch(x)
	if len(matches) != 0 {
		val, ok := ToFloat64(matches[1])
		if ok {
			return val * unitFactors[matches[2]], true
		}
	}

	// try if it looks like an expression in feet via ' "
	matches = feetPattern.FindStringSubmatch(x)
	if len(matches) != 0 {
		feet, ok1 := ToFloat64(matches[1])
		inches, ok2 := ToFloat64(matches[2])

		if ok1 {
			inches += feet * 12.0
		}

		if ok1 || ok2 {
			return inches * metersPerInch, true
		}
	}

	// try and match the first number that can be parsed
	for _, m := range numberPattern.FindAllString(x, 5) {
		if f, ok := ToFloat64(m); ok {
			return f, true
		}
	}

	return 0, false
}

// BuildingHeight computes the height of the building in meters
// given the two tags, "height" and "building:levels".
func BuildingHeight(height, levels string) (float64, bool) {

	// if height is present, and can be parsed as a
	// float, then we can filter right here.
	if h, ok := ToFloat64Meters(height); ok {
		return h, true
	}

	if l, ok := ToFloat64Meters(levels); ok {
		return math.Max(l, 1)*3 + 2, true
	}

	//if height is present, but not numeric, then we have no idea
	// what it could be, and we must assume it could be very large.
	if height != "" || levels != "" {
		return 1.0e10, true
	}

	return 0, false
}

// ReverseLineDirection will reverse the direction of the feature geometry
// if it's a line string.
func ReverseLineDirection(feature *geojson.Feature) bool {
	if ls, ok := feature.Geometry.(orb.LineString); ok {
		ls.Reverse()
		return true
	}

	return false
}

// OneDecimalPoint with convert the value to one decimal point if needed.
func OneDecimalPoint(val float64) string {
	s := fmt.Sprintf("%.1f", val)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

var cardinals = map[string]float64{
	"north": 0, "n": 0, "nne": 22, "ne": 45, "ene": 67,
	"east": 90, "e": 90, "ese": 112, "se": 135, "sse": 157,
	"south": 180, "s": 180, "ssw": 202, "sw": 225, "wsw": 247,
	"west": 270, "w": 270, "wnw": 292, "nw": 315, "nnw": 337,
}

// ToDegrees takes number of directions (e.g. N, NNW) and converts to degrees.
func ToDegrees(x string) (float64, bool) {
	x = strings.ToLower(strings.TrimSpace(x))
	if x == "" {
		return 0, false
	}

	val, ok := ToFloat64(x)
	if ok {
		// always return within range of 0 to 360
		return float64(int(val) % 360), true
	}

	// protect against bad cardinal notations
	c, ok := cardinals[x]
	return c, ok
}
