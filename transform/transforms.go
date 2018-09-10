package transform

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/osmzen/filter"
	"github.com/paulmach/osmzen/util"
	"github.com/paulmach/osmzen/util/streetnames"
)

// Transform makes some kind of change to the feature.
type Transform func(*filter.Context, *geojson.Feature)

// Map converts the name found in the config to the concrete
// Transform function.
func Map(name string) (Transform, bool) {
	name = strings.TrimPrefix(name, "vectordatasource.transform.")
	t, ok := transforms[name]
	return t, ok
}

var transforms = map[string]Transform{
	"tags_create_dict":     nil,
	"tags_name_i18n":       tagsNameI18N, // partially implemented
	"tags_remove":          nil,
	"add_id_to_properties": nil,
	"remove_feature_id":    nil,

	"add_road_network_from_ncat": nil, // south korea road network thing
	"remove_zero_area":           nil,

	// not needed we already do this filter.Context.MinZoom()
	"truncate_min_zoom_to_2dp": nil,

	"detect_osm_relation":              detectOSMRelation,
	"water_tunnel":                     waterTunnel,
	"place_population_int":             placePopulation,
	"calculate_default_place_min_zoom": calculateDefaultPlaceMinZoom,
	"normalize_tourism_kind":           normalizeTourismKind,
	"normalize_operator_values":        normalizeOperatorValues,
	"parse_layer_as_float":             parseLayerAsFloat,
	"road_classifier":                  roadClassifier,
	"road_oneway":                      roadOneway,
	"route_name":                       routeName,
	"road_abbreviate_name":             roadAbbreviateName,
	"normalize_aerialways":             normalizeAerialways,
	"normalize_cycleway":               normalizeCycleway,
	"add_is_bicycle_related":           addIsBicycleRelated,
	"road_trim_properties":             roadTrimProperties,

	"building_height":          buildingHeight,
	"building_min_height":      buildingMinHeight,
	"synthesize_volume":        synthesizeVolume,
	"building_trim_properties": buildingTrimProperties,

	"add_iata_code_to_airports": addIataCodeToAirports,
	"add_uic_ref":               addUICRef,
	"normalize_social_kind":     normalizeSocialKind,
	"normalize_medical_kind":    normalizeMedicalKind,

	"make_representative_point": makeRepresentativePoint,
	"height_to_meters":          heightToMeters,
	"pois_capacity_int":         poisCapacity,
	"elevation_to_meters":       elevationToMeters,
	"admin_level_as_int":        adminLevelAsNum,
}

var (
	// used to detect if an airport's IATA code is the "short"
	// 3-character type. there are also longer codes, and ones
	// which include numbers, but those seem to be used for
	// less important airports.
	iataShortCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

func detectOSMRelation(ctx *filter.Context, feature *geojson.Feature) {
	if feature.Properties.MustString("type", "") == "relation" {
		feature.Properties["osm_relation"] = true
	}
}

func buildingHeight(ctx *filter.Context, feature *geojson.Feature) {
	// we need the height to compute volume so we do it in one place.
	if h := ctx.Height(); h != 0 {
		feature.Properties["height"] = h
	}
}

func buildingMinHeight(ctx *filter.Context, feature *geojson.Feature) {
	height, ok := util.ToFloat64(feature.Properties.MustString("min_height", ""))
	if ok {
		feature.Properties["min_height"] = height
		return
	}

	levels, ok := util.ToFloat64(feature.Properties.MustString("building_min_levels", ""))
	if !ok {
		delete(feature.Properties, "min_height")
		return
	}

	feature.Properties["min_height"] = math.Max(levels, 0) * 3
}

func synthesizeVolume(ctx *filter.Context, feature *geojson.Feature) {
	area := feature.Properties["area"]
	height := feature.Properties["height"]
	if area == nil || height == nil {
		return
	}

	feature.Properties["volume"] = math.Floor(area.(float64) * height.(float64))
}

func buildingTrimProperties(ctx *filter.Context, feature *geojson.Feature) {
	delete(feature.Properties, "building")
	delete(feature.Properties, "building_part")
	delete(feature.Properties, "building_levels")
	delete(feature.Properties, "building_min_levels")
}

func roadClassifier(ctx *filter.Context, feature *geojson.Feature) {
	delete(feature.Properties, "is_link")
	delete(feature.Properties, "is_tunnel")
	delete(feature.Properties, "is_bridge")

	detail := feature.Properties.MustString("kind_detail", "")
	tunnel := feature.Properties.MustString("tunnel", "")
	bridge := feature.Properties.MustString("bridge", "")

	if strings.HasSuffix(detail, "_link") {
		feature.Properties["is_link"] = true
	}

	if tunnel == "yes" || tunnel == "true" {
		feature.Properties["is_tunnel"] = true
	}

	if bridge == "yes" || bridge == "true" {
		feature.Properties["is_bridge"] = true
	}
}

func roadTrimProperties(ctx *filter.Context, feature *geojson.Feature) {
	delete(feature.Properties, "bridge")
	delete(feature.Properties, "tunnel")
}

func roadOneway(ctx *filter.Context, feature *geojson.Feature) {
	oneway := feature.Properties.MustString("oneway", "")
	switch oneway {
	case "-1", "reverse":
		if util.ReverseLineDirection(feature) {
			feature.Properties["oneway"] = "yes"
		}
	case "true", "1":
		feature.Properties["oneway"] = "yes"
	case "false", "0":
		feature.Properties["oneway"] = "no"
	}
}

func routeName(ctx *filter.Context, feature *geojson.Feature) {
	rn := feature.Properties.MustString("route_name", "")
	if rn == "" {
		return
	}

	name := feature.Properties.MustString("name", "")
	if name == "" {
		feature.Properties["name"] = rn
		delete(feature.Properties, "route_name")
	} else if rn == name {
		delete(feature.Properties, "route_name")
	}
}

func placePopulation(ctx *filter.Context, feature *geojson.Feature) {
	pop := feature.Properties.MustString("population", "")
	if f, ok := util.ToFloat64(pop); ok {
		feature.Properties["population"] = math.Floor(f)
	} else {
		delete(feature.Properties, "population")
	}
}

func poisCapacity(ctx *filter.Context, feature *geojson.Feature) {
	capacity := feature.Properties.MustString("capacity", "")
	if f, ok := util.ToFloat64(capacity); ok {
		feature.Properties["capacity"] = math.Floor(f)
	} else {
		delete(feature.Properties, "capacity")
	}
}

func waterTunnel(ctx *filter.Context, feature *geojson.Feature) {
	tunnel := feature.Properties.MustString("tunnel", "")
	delete(feature.Properties, "tunnel")

	if tunnel == "" || tunnel == "no" || tunnel == "false" || tunnel == "0" {
		return
	}

	feature.Properties["is_tunnel"] = true
}

func adminLevelAsNum(ctx *filter.Context, feature *geojson.Feature) {
	level := feature.Properties.MustString("admin_level", "")
	delete(feature.Properties, "admin_level")
	if level == "" {
		return
	}

	if f, ok := util.ToFloat64(level); ok {
		feature.Properties["admin_level"] = math.Floor(f)
	}
}

// place kinds, as used by OSM, mapped to their rough
// min_zoom so that we can provide a defaulted, non-curated min_zoom value.
var defaultMinZoomForPlaceKind = map[string]float64{
	"locality":          13,
	"isolated_dwelling": 13,
	"farm":              13,

	"hamlet": 12,

	"village": 11,

	"suburb":  10,
	"quarter": 10,
	"borough": 10,

	"town": 8,
	"city": 8,

	"province": 4,
	"state":    4,

	"sea": 3,

	"country":   0,
	"ocean":     0,
	"continent": 0,
}

// calculateDefaultPlaceMinZoom
// if the feature does not have a min_zoom attribute already,
// which would have come from a curated source, then calculate
// a default one based on the kind of place it is.
func calculateDefaultPlaceMinZoom(ctx *filter.Context, feature *geojson.Feature) {
	if _, ok := feature.Properties["min_zoom"]; ok {
		return
	}

	// base calculation off kind
	kind := feature.Properties.MustString("kind", "")
	if kind == "" {
		return
	}

	mz, ok := defaultMinZoomForPlaceKind[kind]
	if !ok {
		return
	}

	// adjust min_zoom for state / country capitals
	if kind == "city" || kind == "town" {
		if feature.Properties.MustString("region_capital", "") != "" {
			mz--
		} else if feature.Properties.MustString("country_capital", "") != "" {
			mz -= 2
		}
	}

	feature.Properties["min_zoom"] = mz
}

// roadAbbreviateName
func roadAbbreviateName(ctx *filter.Context, feature *geojson.Feature) {
	name := feature.Properties.MustString("name", "")
	if name == "" {
		return
	}

	feature.Properties["name"] = streetnames.Shorten(name)
}

// If the 'layer' property is present on a feature, then
// this attempts to parse it as a floating point number.
// The old value is removed and, if it could be parsed
// as a floating point number, the number replaces the
// original property.
func parseLayerAsFloat(ctx *filter.Context, feature *geojson.Feature) {
	layer := feature.Properties.MustString("layer", "")
	if layer == "" {
		return
	}

	if f, ok := util.ToFloat64(layer); ok {
		feature.Properties["layer"] = f
	} else {
		delete(feature.Properties, "layer")
	}
}

func normalizeAerialways(ctx *filter.Context, feature *geojson.Feature) {
	aerialway := feature.Properties.MustString("aerialway", "")

	switch aerialway {
	case "cableway":
		// normalise cableway, apparently a deprecated value.
		feature.Properties["aerialway"] = "zip_line"
	case "yes":
		// 'yes' is a pretty unhelpful value, so normalise
		// to a slightly more meaningful 'unknown', which
		// is also a commonly-used value.
		feature.Properties["aerialway"] = "unknown"
	default:
		delete(feature.Properties, "aerialway")
	}
}

// makeRepresentativePoint replaces the geometry of each feature with its
// representative point. This is a point which should be within the interior of
// the geometry, which can be important for labelling concave or doughnut-shaped polygons.
func makeRepresentativePoint(ctx *filter.Context, feature *geojson.Feature) {
	feature.Geometry, _ = planar.CentroidArea(ctx.Geometry)
}

// addIataCodeToAirports
// If the feature is an airport, and it has a 3-character
// IATA code in its tags, then move that code to its
// properties.
func addIataCodeToAirports(ctx *filter.Context, feature *geojson.Feature) {
	kind := feature.Properties["kind"]
	if kind != "aerodrome" && kind != "airport" {
		return
	}

	iata := strings.TrimSpace(ctx.Tags["iata"])
	if iata == "" {
		return
	}

	// IATA codes should be uppercase, and most are, but there
	// might be some in lowercase, so just normalise to upper here.
	iata = strings.ToUpper(iata)
	if iataShortCodePattern.MatchString(iata) {
		feature.Properties["iata"] = iata
	}
}

// addUICRef
// If the feature has a valid uic_ref tag (7 integers), then move it
// to its properties.
func addUICRef(ctx *filter.Context, feature *geojson.Feature) {
	ref := strings.TrimSpace(ctx.Tags["uic_ref"])
	if ref == "" {
		return
	}

	if len(ref) != 7 {
		return
	}

	i, err := strconv.Atoi(ref)
	if err != nil {
		return
	}

	feature.Properties["uic_ref"] = i
}

// normalizeTourismKind
// There are many tourism-related tags, including 'zoo=*' and
// 'attraction=*' in addition to 'tourism=*'. This function promotes
// things with zoo and attraction tags have those values as their
// main kind.
func normalizeTourismKind(ctx *filter.Context, feature *geojson.Feature) {
	zoo := feature.Properties.MustString("zoo", "")
	if zoo != "" {
		feature.Properties["kind"] = zoo
		feature.Properties["tourism"] = "attraction"
		return
	}

	attraction := feature.Properties.MustString("attraction", "")
	if attraction != "" {
		feature.Properties["kind"] = attraction
		feature.Properties["tourism"] = "attraction"
		return
	}
}

// normalizeSocialKind
// Social facilities have an `amenity=social_facility` tag, but more
// information is generally available in the `social_facility=*` tag, so it
// is more informative to put that as the `kind`. We keep the old tag as
// well, for disambiguation.

// Additionally, we normalise the `social_facility:for` tag, which is a
// semi-colon delimited list, to an actual list under the `for` property.
// This should make it easier to consume.
func normalizeSocialKind(ctx *filter.Context, feature *geojson.Feature) {
	kind := feature.Properties.MustString("kind", "")
	if kind != "social_facility" {
		return
	}

	socialFacility := ctx.Tags["social_facility"]
	if socialFacility != "" {
		feature.Properties["kind"] = socialFacility

		// leave the original tag on for disambiguation
		feature.Properties["social_facility"] = socialFacility

		// normalise the 'for' list to an actual list
		if list, ok := ctx.Tags["social_facility:for"]; ok {
			feature.Properties["for"] = strings.Split(list, ";")
		}
	}
}

// normalizeMedicalKind
// Many medical practices, such as doctors and dentists, have a speciality,
// which is indicated through the `healthcare:speciality` tag. This is a
// semi-colon delimited list, so we expand it to an actual list.
func normalizeMedicalKind(ctx *filter.Context, feature *geojson.Feature) {
	kind := feature.Properties.MustString("kind", "")
	if kind == "clinic" || kind == "doctors" || kind == "dentist" {
		speciality := ctx.Tags["healthcare:speciality"]
		if speciality != "" {
			feature.Properties["speciality"] = strings.Split(speciality, ";")
		}
	}
}

// heightToMeters
// If the properties has a "height" entry, then convert that to meters.
func heightToMeters(ctx *filter.Context, feature *geojson.Feature) {

	height := feature.Properties["tags"].(map[string]string)["height"]
	if height == "" {
		return
	}

	if f, ok := util.ToFloat64Meters(height); ok {
		feature.Properties["height"] = f
	} else {
		delete(feature.Properties, "height")
	}
}

// elevationToMeters
// If the properties has an "elevation" entry, then convert that to meters.
func elevationToMeters(ctx *filter.Context, feature *geojson.Feature) {
	elevation := feature.Properties.MustString("elevation", "")
	if elevation == "" {
		return
	}

	if f, ok := util.ToFloat64Meters(elevation); ok {
		feature.Properties["elevation"] = f
	} else {
		delete(feature.Properties, "elevation")
	}
}

// normalizeCycleway
// If the properties contain both a cycleway:left and cycleway:right
// with the same values, those should be removed and replaced with a
// single cycleway property. Additionally, if a cycleway_both tag is
// present, normalize that to the cycleway tag.
func normalizeCycleway(ctx *filter.Context, feature *geojson.Feature) {
	cycleway := feature.Properties.MustString("cycleway", "")
	cyclewayLeft := feature.Properties.MustString("cycleway_left", "")
	cyclewayRight := feature.Properties.MustString("cycleway_right", "")

	cyclewayBoth := feature.Properties.MustString("cycleway_both", "")
	delete(feature.Properties, "cycleway_both")

	if cyclewayBoth != "" && cycleway == "" {
		cycleway = cyclewayBoth
		feature.Properties["cycleway"] = cycleway
	}

	// right and left are the same and there is no cycleway or it's also the same
	// then just have a cycleway tag
	if cyclewayLeft != "" && cyclewayRight != "" &&
		cyclewayLeft == cyclewayRight &&
		(cycleway == "" || cyclewayLeft == cycleway) {

		feature.Properties["cycleway"] = cyclewayLeft
		delete(feature.Properties, "cycleway_right")
		delete(feature.Properties, "cycleway_left")
	}
}

// addIsBicycleRelated
// If the props contain a bicycle_network tag, cycleway, or
// highway=cycleway, it should have an is_bicycle_related
// boolean. Depends on the normalize_cycleway transform to have been
// run first.
func addIsBicycleRelated(ctx *filter.Context, feature *geojson.Feature) {
	delete(feature.Properties, "is_bicycle_related")

	related := false
	if _, ok := feature.Properties["bicycle_network"]; ok {
		related = true
	} else if _, ok := feature.Properties["cycleway"]; ok {
		related = true
	} else if _, ok := feature.Properties["cycleway_left"]; ok {
		related = true
	} else if _, ok := feature.Properties["cycleway_right"]; ok {
		related = true
	} else if feature.Properties.MustString("kind_detail", "") == "cycleway" {
		related = true
	} else if bicycle := feature.Properties.MustString("bicycle", ""); bicycle == "yes" || bicycle == "designated" {
		related = true
	} else if ramp := feature.Properties.MustString("ramp_bicycle", ""); ramp == "yes" || ramp == "left" || ramp == "right" {
		related = true
	}

	if related {
		feature.Properties["is_bicycle_related"] = true
	}
}

var lookupOperatorRules = map[string][]string{
	"United States National Park Service": {
		"National Park Service",
		"US National Park Service",
		"U.S. National Park Service",
		"US National Park service"},
	"United States Forest Service": {
		"US Forest Service",
		"U.S. Forest Service",
		"USDA Forest Service",
		"United States Department of Agriculture",
		"US National Forest Service",
		"United State Forest Service",
		"U.S. National Forest Service"},
	"National Parks & Wildife Service NSW": {
		"Department of National Parks NSW",
		"Dept of NSW National Parks",
		"Dept of National Parks NSW",
		"Department of National Parks NSW",
		"NSW National Parks",
		"NSW National Parks & Wildlife Service",
		"NSW National Parks and Wildlife Service",
		"NSW Parks and Wildlife Service",
		"NSW Parks and Wildlife Service (NPWS)",
		"National Parks and Wildlife NSW",
		"National Parks and Wildlife Service NSW"}}

var normalizedOperatorLookup = map[string]string{}

func init() {
	// flattens the lookupOperatorRules defined above
	for k, v := range lookupOperatorRules {
		for _, s := range v {
			normalizedOperatorLookup[s] = k
		}
	}
}

// normalizeOperatorValues
// There are many operator-related tags, including 'National Park Service',
// 'U.S. National Park Service', 'US National Park Service' etc that refer
// to the same operator tag. This function promotes a normalized value
// for all alternatives in specific operator values.
//
// See https://github.com/tilezen/vector-datasource/issues/927.
func normalizeOperatorValues(ctx *filter.Context, feature *geojson.Feature) {
	operator := feature.Properties.MustString("operator", "")
	if operator == "" {
		return
	}

	normalized := normalizedOperatorLookup[operator]
	if normalized != "" {
		feature.Properties["operator"] = normalized
	}
}

// tagsNameI18N

func tagsNameI18N(ctx *filter.Context, feature *geojson.Feature) {
	if len(ctx.Tags) == 0 || ctx.Tags["name"] == "" {
		return
	}

	// not fully implemented
	// https://github.com/tilezen/vector-datasource/blob/5bb23c587fc5a959c240065268385f0e53b5e34c/vectordatasource/transform.py#L470
	// altNamePrefixCandidates := []string{
	// 	"name:left:",
	// 	"name:right:",
	// 	"name:",
	// 	"alt_name:",
	// 	"old_name:",
	// }

	// name := tags["name"]

	// type lang struct {
	// 	LangResult *langResult
	// 	Value      string
	// }
	// langs := map[string]lang{}

	// for k, v := range ctx.Tags {
	// 	for _, candidate := range altNamePrefixCandidates {
	// 		if strings.HasPrefix(k, candidate) {
	// 			langCode := k[len(candidate):]
	// 			normalizedLangCode := convertOSML10NName(langCode)

	// 			if normalizedLangCode == nil {
	// 				continue
	// 			}

	// 			code := normalizedLangCode.code
	// 			priority := normalizedLangCode.priority
	// 			langKey := candidate + code

	// 			if l, ok := langs[langKey]; !ok || priority < l.LangResult.priority {
	// 				langs[langKey] = lang{
	// 					LangResult: normalizedLangCode,
	// 					Value:      v,
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	// for key, item := range langs {
	// 	feature.Properties[[key] = item.Value
	// }

	name := ctx.Tags["name"]
	for _, altTagNameCandidate := range tagNameAlternates {
		altTagNameValue := ctx.Tags[altTagNameCandidate]
		if altTagNameValue != "" && altTagNameValue != name {
			feature.Properties[altTagNameCandidate] = altTagNameValue
		}
	}
}

var tagNameAlternates = []string{
	"int_name",
	"loc_name",
	"nat_name",
	"official_name",
	"old_name",
	"reg_name",
	"short_name",
	"name_left",
	"name_right",
	"name:short",
}

type langResult struct {
	code     string
	priority int
}

func convertOSML10NName(langCode string) *langResult {
	// TODO: not implemented
	// https://github.com/tilezen/vector-datasource/blob/5bb23c587fc5a959c240065268385f0e53b5e34c/vectordatasource/transform.py
	return nil
}
