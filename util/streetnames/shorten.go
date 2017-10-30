// Package streetnames is utility for handling street names. Based on:
// https://github.com/nvkelso/map-label-style-manual/blob/367233b367a57b2edd0cb170da8b079708580b23/tools/street_names/
package streetnames

import "strings"

var directions = map[string]string{
	"north": "N", "northeast": "NE",
	"east": "E", "southeast": "SE",
	"south": "S", "southwest": "SW",
	"west": "W", "northwest": "NW",
	"n": "N", "ne": "NE",
	"e": "E", "se": "SE",
	"s": "S", "sw": "SW",
	"w": "W", "nw": "NW",
}

var types = map[string]string{
	"ave":        "Ave.",
	"avenue":     "Ave.",
	"blvd":       "Blvd.",
	"boulevard":  "Blvd.",
	"court":      "Ct.",
	"ct":         "Ct.",
	"dr":         "Dr.",
	"drive":      "Dr.",
	"expressway": "Expwy.",
	"expwy":      "Expwy.",
	"freeway":    "Fwy.",
	"fwy":        "Fwy.",
	"highway":    "Hwy.",
	"hwy":        "Hwy.",
	"lane":       "Ln.",
	"ln":         "Ln.",
	"parkway":    "Pkwy.",
	"pkwy":       "Pkwy.",
	"pl":         "Pl.",
	"place":      "Pl.",
	"rd":         "Rd.",
	"road":       "Rd.",
	"st":         "St.",
	"street":     "St.",
	"ter":        "Ter.",
	"terrace":    "Ter.",
	"tr":         "Tr.",
	"trail":      "Tr.",
	"way":        "Wy.",
	"wy":         "Wy.",
}

// Shorten will shorten the a US street name.
// eg. North Expressway Northeast -> North Expwy. NE
func Shorten(name string) string {
	trimmed := strings.TrimSpace(name)
	parts := strings.Fields(trimmed)
	keys := strings.Fields(strings.ToLower(trimmed))

	if len(parts) >= 3 &&
		directions[keys[0]] != "" && // first is a direction
		types[keys[len(keys)-1]] != "" { // last is type
		// like "North Herp Derp Road"
		parts[0] = directions[keys[0]]
		parts[len(parts)-1] = types[keys[len(keys)-1]]
	} else if len(parts) >= 3 &&
		types[keys[len(keys)-2]] != "" && // second to last is a type
		directions[keys[len(keys)-1]] != "" { // last is a direction

		// like "Herp Derp Road North"
		parts[len(parts)-2] = types[keys[len(keys)-2]]
		parts[len(parts)-1] = directions[keys[len(keys)-1]]
	} else if len(parts) >= 2 && types[keys[len(keys)-1]] != "" {
		// like "Herp Derp Road"
		parts[len(parts)-1] = types[keys[len(keys)-1]]
	}

	return strings.Join(parts, " ")
}
