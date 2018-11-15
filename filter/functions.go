package filter

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osmzen/util"
	"github.com/pkg/errors"
)

var functions map[string]func([]Expression) (Expression, error)

func init() {
	functions = map[string]func([]Expression) (Expression, error){
		"mz_building_kind_detail":       compileBuildingKindDetail,
		"mz_building_part_kind_detail":  compileBuildingPartKindDetail,
		"mz_calculate_path_major_route": compileCalculatePathMajorRoute,
		"mz_calculate_ferry_level":      compileCalculateFerryLevel,
		"mz_calculate_is_bus_route":     compileCalculateIsBusRoute, // *** needs a look
		"mz_cycling_network":            compileCyclingNetwork,      // *** now a column
		// "mz_hiking_network":                  compileHikingNetwork,
		// "mz_get_rel_networks":                compileGetRelNetworks,
		"mz_to_float_meters":                 compileToFloatMeters,
		"mz_get_min_zoom_highway_level_gate": compileGetMinZoomHighwayLevelGate,
		"util.safe_int":                      compileSafeInt,
		"util.tag_str_to_bool":               compileTagStrToBool,
		"util.true_or_none":                  compileTrueOrNone,
		"util.is_building":                   compileCalculateIsBuildingOrPart,
	}
}

type toFloatMeters struct {
	Args []Expression
}

func (f *toFloatMeters) Eval(ctx *Context) interface{} {
	raw := f.Args[0].Eval(ctx)
	if raw == nil {
		return nil
	}

	return f.EvalNum(ctx)
}

func (f *toFloatMeters) EvalNum(ctx *Context) float64 {
	raw := f.Args[0].Eval(ctx)
	if raw == nil {
		// TODO: no value, return 0?
		return 0
	}

	switch val := raw.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}

	val, ok := raw.(string)
	if !ok {
		panic(fmt.Sprintf("to_float_meters: value is not valid: (%T, %v)", raw, raw))
	}

	v, _ := util.ToFloat64Meters(val)

	// v could be zero if it's not a valid length, e.g. a word
	return v
}

func compileToFloatMeters(args []Expression) (Expression, error) {
	return &toFloatMeters{Args: args}, nil
}

type calculateFerryLevel struct{}

func (f calculateFerryLevel) Eval(ctx *Context) interface{} {
	return f.EvalNum(ctx)
}

// mz_calculate_ferry_level
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L1-L17
func (f calculateFerryLevel) EvalNum(ctx *Context) float64 {
	if t := ctx.Geometry.GeoJSONType(); t != geojson.TypeLineString && t != geojson.TypeMultiLineString {
		if ctx.Verbose {
			log.Printf("failed to calculate ferry level: %v is non-line", ctx.FeatureID)
		}

		return 0.0
	}

	length := ctx.Length()

	// about when the way is >= 2px in length
	if length > 1224 {
		return 8.0
	} else if length > 611 {
		return 9.0
	} else if length > 306 {
		return 10.0
	} else if length > 153 {
		return 11.0
	} else if length > 76 {
		return 12.0
	}

	return 13.0
}

func compileCalculateFerryLevel(args []Expression) (Expression, error) {
	return calculateFerryLevel{}, nil
}

type getMinZoomHighwayLevelGate struct{}

func (f getMinZoomHighwayLevelGate) Eval(ctx *Context) interface{} {
	return f.EvalNum(ctx)
}

// Identifies and returns the min_zoom for gates
// given the highway level of gate location
// mz_get_min_zoom_highway_level_gate
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L901-L925
func (f getMinZoomHighwayLevelGate) EvalNum(ctx *Context) float64 {
	zoom := 0.0

	for _, w := range ctx.wayMembership() {
		z := 17.0

		highway := w.Tags.Find("highway")
		if stringIn(highway, []string{"motorway", "trunk", "primary", "motorway_link", "trunk_link", "primary_link"}) {
			z = 14
		} else if stringIn(highway, []string{"secondary", "tertiary", "secondary_link", "tertiary_link"}) {
			z = 15
		} else if stringIn(highway, []string{"residential", "service", "path", "track", "footway", "unclassified"}) {
			z = 16
		}

		if z > zoom {
			zoom = z
		}
	}

	if zoom == 0 {
		return 17.0
	}

	return zoom
}

func compileGetMinZoomHighwayLevelGate(args []Expression) (Expression, error) {
	return getMinZoomHighwayLevelGate{}, nil
}

type calculateIsBusRoute struct{}

// mz_calculate_is_bus_route
// https://github.com/tilezen/vector-datasource/blob/d28bc2801e808e02b48023e165c8664ebe4c0486/data/functions.sql#L547-L563
func (f calculateIsBusRoute) Eval(ctx *Context) interface{} {
	for _, r := range ctx.relationMembership() {
		if r.Tags.Find("type") == "route" {
			route := r.Tags.Find("route")
			if route == "bus" || route == "trolleybus" {
				return true
			}
		}
	}

	return nil
}

func compileCalculateIsBusRoute(args []Expression) (Expression, error) {
	return calculateIsBusRoute{}, nil
}

type hikingNetwork struct{}

// Looks up whether the given osm_id is a member of any hiking routes
// and, if so, returns the network designation of the most important
// (highest in hierarchy) of the networks.

// mz_hiking_network
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L580-L597
func (f hikingNetwork) Eval(ctx *Context) interface{} {
	counts := [3]int{}

	for _, r := range ctx.relationMembership() {
		tm := r.Tags.Map()
		if !isPathMajorRouteRelation(tm) {
			continue
		}

		network := tm["network"]
		switch network {
		case "iwn":
			return "iwn"
		case "nwn":
			counts[0]++
		case "rwn":
			counts[1]++
		case "lwn":
			counts[2]++
		}
	}

	if counts[0] != 0 {
		return "nwn"
	}

	if counts[1] != 0 {
		return "rwn"
	}

	if counts[2] != 0 {
		return "lwn"
	}

	return nil
}

func compileHikingNetwork(args []Expression) (Expression, error) {
	return hikingNetwork{}, nil
}

// mz_cycling_network_
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L599-L619
func cyclingNetworkHelper(ctx *Context) interface{} {
	counts := [3]int{}

	for _, r := range ctx.relationMembership() {
		tm := r.Tags.Map()
		if !isPathMajorRouteRelation(tm) {
			continue
		}

		network := tm["network"]
		switch network {
		case "icn":
			return "icn"
		case "ncn":
			counts[0]++
		case "rcn":
			counts[1]++
		case "lcn":
			counts[2]++
		}
	}

	if counts[0] != 0 {
		return "ncn"
	}

	if ctx.Tags["ncn"] == "yes" || ctx.Tags["ncn_ref"] != "" {
		return "ncn"
	}

	if counts[1] != 0 {
		return "rcn"
	}

	if ctx.Tags["rcn"] == "yes" || ctx.Tags["rcn_ref"] != "" {
		return "rcn"
	}

	if counts[2] != 0 {
		return "lcn"
	}

	if ctx.Tags["lcn"] == "yes" || ctx.Tags["lcn_ref"] != "" {
		return "lcn"
	}

	return nil
}

type cyclingNetwork struct{}

// mz_cycling_network
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L621-L629
func (f cyclingNetwork) Eval(ctx *Context) interface{} {
	if ctx.Tags["icn"] == "yes" || ctx.Tags["icn_ref"] != "" {
		return "icn"
	}

	return cyclingNetworkHelper(ctx)
}

func compileCyclingNetwork(args []Expression) (Expression, error) {
	return cyclingNetwork{}, nil
}

type getRelNetworks struct{}

// mz_get_rel_networks returns a list of triples of route type,
// network and ref tags, or NULL, for a given way ID.
//
// it does this by joining onto the relations slim table, so it
// won't work if you dropped the slim tables, or didn't use slim
// mode in osm2pgsql.
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L93-L114
func (f getRelNetworks) Eval(ctx *Context) interface{} {
	relations := ctx.relationMembership()
	if len(relations) == 0 {
		return nil
	}

	result := make([]string, 0, len(relations))
	for _, r := range relations {
		route := r.Tags.Find("route")
		network := r.Tags.Find("network")
		ref := r.Tags.Find("ref")

		if route != "" && (network != "" || ref != "") {
			result = append(result, route, network, ref)
		}
	}

	return result
}

func compileGetRelNetworks(args []Expression) (Expression, error) {
	return getRelNetworks{}, nil
}

// mz_is_path_major_route_relation
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L565-L574
func isPathMajorRouteRelation(tags map[string]string) bool {
	// this should input a relation's tags.
	return tags["type"] == "route" &&
		stringIn(tags["route"], []string{"hiking", "foot", "bicycle"}) &&
		stringIn(tags["network"], []string{"iwn", "nwn", "rwn", "lwn", "icn", "ncn", "rcn", "lcn"})
}

type calculatePathMajorRoute struct{}

func (f calculatePathMajorRoute) Eval(ctx *Context) interface{} {
	return f.EvalNum(ctx)
}

// mz_calculate_path_major_route
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L635-L655
func (f calculatePathMajorRoute) EvalNum(ctx *Context) float64 {
	zoom := 20.0

	for _, r := range ctx.relationMembership() {
		tm := r.Tags.Map()
		if !isPathMajorRouteRelation(tm) {
			continue
		}

		z := 20.0
		network := tm["network"]
		if stringIn(network, []string{"icn", "ncn"}) {
			z = 8
		} else if stringIn(network, []string{"iwn", "nwn"}) {
			z = 9
		} else if network == "rcn" {
			z = 10
		} else if network == "rwn" {
			z = 11
		} else if network == "lcn" {
			z = 11
		} else if network == "lwn" {
			z = 12
		}

		if z < zoom {
			zoom = z
		}
	}

	return zoom
}

func compileCalculatePathMajorRoute(args []Expression) (Expression, error) {
	return calculatePathMajorRoute{}, nil
}

type safeInt struct {
	Arg Expression
}

func (f safeInt) Eval(ctx *Context) interface{} {
	val := f.Arg.Eval(ctx)
	if val == nil {
		return nil
	}

	if v, ok := val.(int); ok && v == 0 {
		return nil
	}

	if v, ok := val.(float64); ok && v == 0 {
		return nil
	}

	return val
}

type safeIntNum struct {
	Arg NumExpression
}

func (f safeIntNum) Eval(ctx *Context) interface{} {
	val := f.Arg.EvalNum(ctx)
	if val == 0 {
		return nil
	}

	return val
}

func compileSafeInt(args []Expression) (Expression, error) {
	if len(args) < 1 {
		return nil, errors.Errorf("safe_int requires 1 arg")
	}

	if a, ok := args[0].(NumExpression); ok {
		return safeIntNum{a}, nil
	}
	return safeInt{args[0]}, nil
}

type tagStrToBool struct {
	Arg Expression
}

func (f tagStrToBool) Eval(ctx *Context) interface{} {
	val := f.Arg.Eval(ctx)

	str, ok := val.(string)
	if !ok {
		return nil
	}

	str = strings.ToLower(str)
	if str == "yes" || str == "true" {
		return true
	}

	return nil
}

func compileTagStrToBool(args []Expression) (Expression, error) {
	if len(args) < 1 {
		return nil, errors.Errorf("tag_str_to_bool requires 1 arg")
	}

	return tagStrToBool{args[0]}, nil
}

type trueOrNone struct {
	Arg Expression
}

func (f trueOrNone) Eval(ctx *Context) interface{} {
	val := f.Arg.Eval(ctx)

	b, ok := val.(bool)
	if !ok && !b {
		return nil
	}

	return true
}

func compileTrueOrNone(args []Expression) (Expression, error) {
	if len(args) < 1 {
		return nil, errors.Errorf("true_or_none requires 1 arg")
	}

	return trueOrNone{args[0]}, nil
}

type calculateIsBuildingOrPart struct{}

// mz_calculate_is_building_or_part
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L19-L29
func (f calculateIsBuildingOrPart) Eval(ctx *Context) interface{} {
	// there are 12,000 uses of building=no, so we ought to take that into
	// account when figuring out if something is a building or not. also,
	// returning "kind=no" is a bit weird.
	building := ctx.Tags["building"]
	if building != "" && building != "no" {
		return true
	}

	part := ctx.Tags["building:part"]
	if part != "" && part != "no" {
		return true
	}

	return nil
}
func compileCalculateIsBuildingOrPart(args []Expression) (Expression, error) {
	return calculateIsBuildingOrPart{}, nil
}

// mz_building_height
// Calculate the height of a building by looking at either the
// height tag, if one is set explicitly, or by calculating the
// approximate height from the number of levels, if that is set.
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L504-L526
func buildingHeight(ctx *Context) float64 {
	height := ctx.Tags["height"]
	levels := ctx.Tags["building:levels"]

	// if height is present, and can be parsed as a
	// float, then we can filter right here.
	if height != "" {
		if f, ok := util.ToFloat64Meters(height); ok {
			return f
		}

		// if height is present, but not numeric, then we have no idea
		// what it could be, and we must assume it could be very large.
		return 1.0e10
	}

	// looks like we assume each level is 3m, plus 2 overall.
	if levels != "" {
		f, err := strconv.ParseFloat(levels, 64)
		if err == nil {
			return math.Max(f, 1)*3 + 2
		}

		return 1.0e10
	}

	return 0
}

// mz_building_kind_detail
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L702-L863
var buildingKindDetailMap = buildKindMap(
	[]string{
		"bangunan", "building", "other", "rumah", "Rumah", "Rumah Masyarakat",
		"rumah_penduduk", "true", "trullo", "yes"},
	[]string{
		"abandoned", "administrative", "agricultural", "airport", "allotment_house",
		"apartments", "arbour", "bank", "barn", "basilica", "beach_hut", "bell_tower",
		"boathouse", "brewery", "bridge", "bungalow", "bunker", "cabin", "carport",
		"castle", "cathedral", "chapel", "chimney", "church", "civic", "clinic",
		"clubhouse", "collapsed", "college", "commercial", "construction", "container",
		"convent", "cowshed", "dam", "damaged", "depot", "destroyed", "detached",
		"disused", "dormitory", "duplex", "factory", "farm", "farm_auxiliary",
		"fire_station", "garage", "garages", "gazebo", "ger", "glasshouse", "government",
		"grandstand", "greenhouse", "hangar", "healthcare", "hermitage", "hospital",
		"hotel", "house", "houseboat", "hut", "industrial", "kindergarten", "kiosk",
		"library", "mall", "manor", "manufacture", "mobile_home", "monastery",
		"mortuary", "mosque", "museum", "office", "outbuilding", "parking", "pavilion",
		"power", "prison", "proposed", "pub", "public", "residential", "restaurant",
		"retail", "roof", "ruin", "ruins", "school", "semidetached_house", "service",
		"shed", "shelter", "shop", "shrine", "silo", "slurry_tank", "stable", "stadium",
		"static_caravan", "storage", "storage_tank", "store", "substation",
		"summer_cottage", "summer_house", "supermarket", "synagogue", "tank", "temple",
		"terrace", "tower", "train_station", "transformer_tower", "transportation",
		"university", "utility", "veranda", "warehouse", "wayside_shrine", "works"},
	map[string]string{
		"barne":                   "barn",
		"commercial;residential":  "mixed_use",
		"constructie":             "construction",
		"dwelling_house":          "house",
		"education":               "school",
		"greenhouse_horticulture": "greenhouse",

		"apartment": "apartments",
		"flat":      "apartments",

		"houses":               "residential",
		"residences":           "residential",
		"residence":            "residential",
		"perumahan permukiman": "residential",
		"residentiel1":         "residential",
		"offices":              "office",
		"prefab_container":     "container",
		"public_building":      "public",
		"railway_station":      "train_station",
		"roof=permanent":       "roof",
		"stables":              "stable",
		"static caravan":       "static_caravan",
		"station":              "transportation",
		"storage tank":         "storage_tank",
		"townhome":             "terrace"},
)

type buildingKindDetail struct{}

func (f buildingKindDetail) Eval(ctx *Context) interface{} {
	key := ctx.Tags["building"]
	if val := buildingKindDetailMap[key]; val != "" {
		return val
	}

	return nil
}

func compileBuildingKindDetail(args []Expression) (Expression, error) {
	return buildingKindDetail{}, nil
}

// mz_building_part_kind_detail
// https://github.com/tilezen/vector-datasource/blob/617f2011d262b6f2171e988fd60931890663cf7a/data/functions.sql#L865-L899
var buildingPartKindDetailMap = buildKindMap(
	[]string{"yes", "part", "church:part", "default"},
	[]string{
		"arch", "balcony", "base", "column", "door", "elevator", "entrance", "floor",
		"hall", "main", "passageway", "pillar", "porch", "ramp", "roof", "room",
		"steps", "stilobate", "tier", "tower", "verticalpassage", "wall", "window"},
	map[string]string{
		"corridor":        "verticalpassage",
		"Corridor":        "verticalpassage",
		"vertical":        "verticalpassage",
		"verticalpassage": "verticalpassage",

		"stairs":   "steps",
		"stairway": "steps"},
)

type buildingPartKindDetail struct{}

func (f buildingPartKindDetail) Eval(ctx *Context) interface{} {
	key := ctx.Tags["building:part"]
	if val := buildingPartKindDetailMap[key]; val != "" {
		return val
	}

	return nil
}

func compileBuildingPartKindDetail(args []Expression) (Expression, error) {
	return buildingPartKindDetail{}, nil
}

func buildKindMap(empty, same []string, kv map[string]string) map[string]string {
	for _, s := range empty {
		kv[s] = ""
	}

	for _, s := range same {
		kv[s] = s
	}

	return kv
}

func parseFloat64(arg interface{}) (float64, error) {
	if i, ok := arg.(int); ok {
		return float64(i), nil
	}

	if f, ok := arg.(float64); ok {
		return f, nil
	}

	s, ok := arg.(string)
	if !ok {
		return 0, errors.Errorf("not a number: (%T, %v", arg, arg)
	}

	v, err := strconv.ParseFloat(s, 64)
	return v, errors.WithStack(err)
}

func stringIn(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}

	return false
}
