package transform

import (
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osmzen/filter"
	"github.com/paulmach/osmzen/util"
)

type restriction struct {
	kind   string
	format func(string) (bool, string)
}

var restrictions = map[string]*restriction{
	"maxwidth":    &restriction{"width", restrictionMetresFormat},
	"maxlength":   &restriction{"length", restrictionMetresFormat},
	"maxheight":   &restriction{"height", restrictionMetresFormat},
	"maxweight":   &restriction{"weight", restrictionTonnesFormat},
	"maxaxleload": &restriction{"wpa", restrictionTonnesFormat},
	"hazmat":      &restriction{"hazmat", restrictionFalseFormat},
}

func restrictionMetresFormat(val string) (bool, string) {
	// parse metres or feet and inches, return cm
	if metres, ok := util.ToFloat64Meters(val); ok {
		return true, util.OneDecimalPoint(metres) + "m"
	}
	return false, ""
}

func restrictionTonnesFormat(val string) (bool, string) {
	if tonnes, ok := util.ToFloat64(val); ok {
		return true, util.OneDecimalPoint(tonnes) + "t"
	}
	return false, ""
}

func restrictionFalseFormat(val string) (bool, string) {
	return val == "no", ""
}

// Parse the maximum height, weight, length, etc... restrictions on vehicles
// and create the `hgv_restriction` and `hgv_restriction_shield_text`.
// https://github.com/tilezen/vector-datasource/blob/master/vectordatasource/transform.py#L8755-L8821
func addVehicleRestrictions(ctx *filter.Context, feature *geojson.Feature) {
	var hgvRestriction = ""
	var hgvRestrictionShieldText = ""

	for key, restriction := range restrictions {
		// TODO: maybe not use Must here?
		val := feature.Properties.MustString(key, "")
		if val == "" {
			continue
		}

		restricted, shieldText := restriction.format(val)
		if !restricted {
			continue
		}

		if hgvRestriction == "" {
			hgvRestriction = restriction.kind
			hgvRestrictionShieldText = shieldText
		} else {
			hgvRestriction = "multiple"
			hgvRestrictionShieldText = ""
		}
	}

	if hgvRestriction != "" {
		feature.Properties["hgv_restriction"] = hgvRestriction
	}
	if hgvRestrictionShieldText != "" {
		feature.Properties["hgv_restriction_shield_text"] = hgvRestrictionShieldText
	}
}
