package postprocess

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/paulmach/osmzen/filter"
	"github.com/paulmach/osmzen/matcher"
	"github.com/paulmach/osmzen/ranker"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/clip/smartclip"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"

	"github.com/pkg/errors"
)

var functions = map[string]func(*CompileContext, *Config) (Function, error){
	// functions defined in tilezen/vector-datasource.
	// nil values have not been implemented.
	"numeric_min_filter":                 compileNumericMinFilter,
	"road_networks":                      compileRoadNetworks,
	"build_fence":                        nil,
	"drop_properties":                    nil, // TODO
	"csv_match_properties":               compileCSVMatchProperties,
	"exterior_boundaries":                nil,
	"drop_features_mz_min_pixels":        nil,
	"overlap":                            nil, // look into
	"admin_boundaries":                   nil,
	"apply_disputed_boundary_viewpoints": nil,
	"drop_names_on_short_boundaries":     nil,
	"handle_label_placement":             compileHandleLabelPlacement,
	"remove_duplicate_features":          compileRemoveDuplicateFeatures,
	"drop_features_where":                compileDropFeaturesWhere,
	"merge_line_features":                nil,
	"merge_building_features":            nil,
	"merge_polygon_features":             nil,
	"generate_address_points":            nil,
	"merge_duplicate_stations":           nil,
	"normalize_station_properties":       nil,
	"rank_features":                      nil,
	"update_parenthetical_properties":    compileUpdateParentheticalProperties,
	"keep_n_features":                    nil,
	"drop_properties_with_prefix":        nil,
	"drop_small_inners":                  nil,
	"simplify_and_clip":                  nil,
	"intercut":                           nil,
	"simplify_layer":                     nil,
	"backfill_from_other_layer":          compileBackfillFromOtherLayers,
	"buildings_unify":                    nil,
	"palettize_colours":                  nil,
	"point_in_country_logic":             nil,
	"tags_set_ne_min_max_zoom":           nil,
	"drop_layer":                         nil, // drops admin_areas layer, which we completely ignore
	"max_zoom_filter":                    nil, // Natural Earth layers, drop if out of range
	"min_zoom_filter":                    nil,
	"update_min_zoom":                    nil,              // TODO
	"remap":                              compileRemap,     // only hits zoom 13 on landue
	"drop_names":                         compileDropNames, // only hits zoom 13 on landuse
	"whitelist":                          compileWhitelist,
	"quantize_height":                    compileQuantizeHeight,
	"clamp_min_zoom":                     compileClampMinZoom,
	"add_collision_rank":                 compileAddCollisionRank,
}

var (
	// used to detect if the "name" of a building is
	// actually a house number.
	digitsPattern = regexp.MustCompile(`^[0-9-]+$`)

	// used to detect station names which are followed by a
	// parenthetical list of line names.
	stationPattern = regexp.MustCompile(`([^(]*)\(([^)]*)\).*`)
)

// SetConditionalNames sets names for building and other layers based on
// feature properties. In the original tilezen/vector-datasource this is
// done when loading the data into postgres.
func SetConditionalNames(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	// in the original queries/buildings.jinja2 it basically does this,
	// set the name to addr:housename if the building is in the pois or landuse layer.
	// Some info here: https://github.com/tilezen/vector-datasource/issues/201
	// Example is way 133113873, which is a amenity=school and building=yes.
	// We want it to
	// - have a building polygon, no label, achieved because of building=yes,
	//   in POI layer, and it doesn't have a addr:housename,
	// - have a POI for the label, achieved because amenity=school,
	// - not in landuse layer, achieved because in poi layer

	buildings := layers["buildings"]
	pois := layers["pois"]
	landuse := layers["landuse"]

	if buildings != nil {
	buildings:
		for _, feature := range buildings.Features {
			// https://github.com/tilezen/vector-datasource/blob/001a0549345bab57f471a4dc08366852453b0770/queries/buildings.jinja2#L2-L10

			ftype := feature.Properties["type"]
			fid := feature.Properties["id"]

			if pois != nil {
				for _, poi := range pois.Features {
					if fid == poi.Properties["id"] && ftype == poi.Properties["type"] {
						hn := feature.Properties["tags"].(map[string]string)["addr:housename"]
						if hn != "" {
							feature.Properties["name"] = hn
						} else {
							delete(feature.Properties, "name")
						}
						continue buildings
					}
				}
			}

			if landuse != nil {
				for _, land := range landuse.Features {
					if fid == land.Properties["id"] && ftype == land.Properties["type"] {
						hn := feature.Properties["tags"].(map[string]string)["addr:housename"]
						if hn != "" {
							feature.Properties["name"] = hn
						} else {
							delete(feature.Properties, "name")
						}
						continue buildings
					}
				}
			}
		}
	}

	if landuse != nil {
		for _, feature := range landuse.Features {
			// https://github.com/tilezen/vector-datasource/blob/001a0549345bab57f471a4dc08366852453b0770/queries/landuse.jinja2#L14-L15
			ftype := feature.Properties["type"]
			fid := feature.Properties["id"]

			if pois != nil {
				for _, poi := range pois.Features {
					if fid == poi.Properties["id"] && ftype == poi.Properties["type"] {
						delete(feature.Properties, "name")
						break
					}
				}
			}
		}
	}
}

func any(vals []bool) bool {
	for _, v := range vals {
		if v {
			return v
		}
	}

	return false
}

func all(vals []bool) bool {
	for _, v := range vals {
		if !v {
			return false
		}
	}

	return true
}

type csvMatchProperties struct {
	SourceLayer string
	Matcher     *matcher.Matcher
}

func (f *csvMatchProperties) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	features := layers[f.SourceLayer]
	if features == nil {
		return
	}

	mctx := &matcher.Context{
		Zoom: ctx.Zoom,
	}
	for _, feature := range features.Features {
		f.Matcher.Eval(mctx, feature)
	}
}

func compileCSVMatchProperties(ctx *CompileContext, c *Config) (Function, error) {
	data, err := ctx.Asset(c.Resources.Matcher.Path)
	if err != nil {
		return nil, err
	}

	m, err := matcher.Load(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return &csvMatchProperties{
		SourceLayer: c.Params["source_layer"].(string),
		Matcher:     m,
	}, nil
}

type addCollisionRank struct {
	Ranker *ranker.Ranker
}

func (f *addCollisionRank) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	for name, layer := range layers {
		for _, feature := range layer.Features {

			// hard coded version of the where clause in queries.yaml
			add := false
			if name == "pois" || hasName(feature) {
				add = true
			} else if _, ok := feature.Properties["ref"]; ok {
				add = true
			} else if _, ok := feature.Properties["shield_text"]; ok {
				add = true
			} else if _, ok := feature.Properties["bicycle_shield_text"]; ok {
				add = true
			} else if _, ok := feature.Properties["bus_shield_text"]; ok {
				add = true
			} else if _, ok := feature.Properties["walking_shield_text"]; ok {
				add = true
			} else if _, ok := feature.Properties["bicycle_shield_text"]; ok {
				add = true
			}

			if add {
				rank := f.Ranker.Rank(name, feature.Properties)
				feature.Properties["collision_rank"] = rank
			}
		}
	}
}

func compileAddCollisionRank(ctx *CompileContext, c *Config) (Function, error) {
	data, err := ctx.Asset(c.Resources.Ranker.Path)
	if err != nil {
		return nil, err
	}

	r, err := ranker.Load(data)
	if err != nil {
		return nil, err
	}

	where := "layer_name == 'pois' or " +
		"_has_name or " +
		"ref is not None or " +
		"shield_text is not None or " +
		"bicycle_shield_text is not None or " +
		"bus_shield_text is not None or " +
		"walking_shield_text is not None"
	if where != c.Params["where"] {
		return nil, errors.Errorf("add_collision_rank: where has changed, it's hard coded")
	}

	return &addCollisionRank{
		Ranker: r,
	}, nil
}

type handleLabelPlacement struct {
	Layers      []string
	ClipFactors map[string]float64
	StartZoom   float64
	Condition   filter.Condition
}

func (f *handleLabelPlacement) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom {
		return
	}

	for _, l := range f.Layers {
		layer := layers[l]
		if layer != nil {
			paddedBound := padBoundByFactor(ctx.Bound, f.ClipFactors[l])
			f.evalLayer(ctx, paddedBound, layer)
		}
	}
}

func (f *handleLabelPlacement) evalLayer(ctx *Context, paddedBound orb.Bound, layer *geojson.FeatureCollection) {
	end := len(layer.Features)
	for i := 0; i < end; i++ {
		feature := layer.Features[i]

		ctx.fctx = filter.NewContextFromProperties(ctx.fctx, feature.Properties)
		if !f.Condition.Eval(ctx.fctx) {
			continue
		}

		if hasOpenOuterRing(feature.Geometry) {
			// if we have a polygon with an open outer ring taking the centroid
			// will definitely not be within the polygon. We crop and wrap it first
			// so it's closed around the bound the correct way. This will give us better
			// odds of having a good label placement.
			clipToBound(ctx.Bound, feature)
			if feature.Geometry == nil {
				// geometry is not in the bound, odd, ignore.
				continue
			}
		}

		centroid, _ := planar.CentroidArea(feature.Geometry)
		if !paddedBound.Contains(centroid) {
			continue
		}

		nf := geojson.NewFeature(centroid)
		nf.Properties = feature.Properties.Clone()
		nf.Properties["label_placement"] = true

		layer.Features = append(layer.Features, nf)
	}
}

func compileHandleLabelPlacement(ctx *CompileContext, c *Config) (Function, error) {
	f := &handleLabelPlacement{}
	if c.Params["layers"] != nil {
		f.Layers = parseStrings(c.Params["layers"])
	}

	if c.Params["start_zoom"] != nil {
		f.StartZoom = float64(c.Params["start_zoom"].(int))
	}

	if c.Params["label_where"] != nil {
		cond, err := filter.CompileCondition(c.Params["label_where"])
		if err != nil {
			return nil, err
		}

		f.Condition = cond
	}

	f.ClipFactors = ctx.ClipFactors
	return f, nil
}

type numericMinFilter struct {
	Layer   string
	Mode    string
	Filters map[int]map[string]float64
}

func (f *numericMinFilter) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	// Keep only features which have properties equal or greater
	// than the configured minima. These are in a dict per zoom
	// like this:
	// { 15: { 'area': 1000 }, 16: { 'area': 2000 } }
	// This would mean that at zooms 15 and 16, the filter was
	// active. At other zooms it would do nothing.
	// Multiple filters can be given for a single zoom. The
	// `mode` parameter can be set to 'any' to require that only
	// one of the filters needs to match, or any other value to
	// use the default 'all', which requires all filters to
	// match.

	minima := f.Filters[int(ctx.Zoom)]
	if minima == nil {
		// no filtering for this zoom level
		return
	}

	fc := layers[f.Layer]
	if fc == nil {
		// layer not part of this request
		return
	}

	aggFunc := all
	if f.Mode == "any" {
		aggFunc = any
	}

	at := 0
	for _, f := range fc.Features {
		keep := make([]bool, 0, len(minima))
		for prop, min := range minima {
			// note: if the property is not defined the filter will be skipped
			if f.Properties.MustFloat64(prop, min) >= min {
				keep = append(keep, true)
			} else {
				keep = append(keep, false)
			}
		}

		if aggFunc(keep) {
			fc.Features[at] = f
			at++
		}
	}

	fc.Features = fc.Features[:at]
}

func compileNumericMinFilter(ctx *CompileContext, c *Config) (Function, error) {
	f := &numericMinFilter{
		Mode:    "all",
		Filters: make(map[int]map[string]float64),
	}

	if c.Params["source_layer"] != nil {
		f.Layer = c.Params["source_layer"].(string)
	}

	if c.Params["mode"] != nil {
		f.Mode = c.Params["mode"].(string)
	}

	if c.Params["filters"] != nil {
		filters := c.Params["filters"].(map[interface{}]interface{})
		for k, v := range filters {
			key, ok := k.(int)
			if !ok {
				return nil, errors.Errorf("numeric_min_filter: filter key must be integer zoom: (%T, %v)", k, k)
			}

			f.Filters[key] = make(map[string]float64)
			for prop, min := range v.(map[interface{}]interface{}) {
				switch min := min.(type) {
				case int:
					f.Filters[key][prop.(string)] = float64(min)
				case float64:
					f.Filters[key][prop.(string)] = min
				default:
					return nil, errors.Errorf("numeric_min_filter: filter property not a number: (%T, %v)", min, min)
				}
			}
		}
	}

	return f, nil
}

type removeDuplicateFeatures struct {
	Layers        []string
	Keys          []string
	GeometryTypes []string
	EndZoom       float64
	MinDistance   float64 // Pixel distance to remove duplicates
}

type deduplicator struct {
	Distance float64
	Parts    []string
	Keys     []string
	Found    map[string][]*geojson.Feature
}

func (d *deduplicator) Keep(feature *geojson.Feature) bool {
	for i, k := range d.Keys {
		s := feature.Properties.MustString(k, "")
		if s == "" {
			// if we're missing, we should keep it.
			return true
		}

		d.Parts[i] = s
	}

	// NOTE: if the string "-!-" is in part of the keys we could have a bug.
	key := strings.Join(d.Parts, "-!-")
	features, ok := d.Found[key]
	if !ok {
		d.Found[key] = append(d.Found[key], feature)
		return true
	}

	point := feature.Geometry.(orb.Point)
	for _, f := range features {
		dist := geo.Distance(point, f.Geometry.(orb.Point))
		if dist < d.Distance {
			return false
		}
	}

	d.Found[key] = append(d.Found[key], feature)
	return true
}

func (f *removeDuplicateFeatures) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if f.EndZoom != 0 && ctx.Zoom > f.EndZoom {
		return
	}

	// f.MinDistance is pixel distance at the tile.
	// This is complicate code to figure out an approximate that geo.Distance

	// convert corner of tile to pixels
	tile := maptile.At(ctx.Bound.Min, maptile.Zoom(ctx.Zoom+8))
	p1 := tile.Center()

	// move pixel distance and convert back to geo point
	tile.X += uint32(f.MinDistance)

	// find distance from corner to new point
	dist := geo.Distance(p1, tile.Center())

	deduper := &deduplicator{
		Distance: dist,
		Parts:    make([]string, len(f.Keys)),
		Keys:     f.Keys,
		Found:    make(map[string][]*geojson.Feature),
	}

	for _, ln := range f.Layers {
		fc := layers[ln]
		if fc == nil {
			return
		}

		index := 0
		for _, feature := range fc.Features {
			if !stringIn(feature.Geometry.GeoJSONType(), f.GeometryTypes) {
				fc.Features[index] = feature
				index++
				continue
			}

			if deduper.Keep(feature) {
				fc.Features[index] = feature
				index++
			}
		}

		fc.Features = fc.Features[:index]
	}
}

func compileRemoveDuplicateFeatures(ctx *CompileContext, c *Config) (Function, error) {
	f := &removeDuplicateFeatures{}

	if c.Params["source_layer"] != nil {
		f.Layers = []string{
			c.Params["source_layer"].(string),
		}
	}

	if c.Params["source_layers"] != nil {
		if len(f.Layers) > 0 {
			return nil, errors.New("remove_duplicate_features: must define source_layer XOR source_layers")
		}

		f.Layers = parseStrings(c.Params["source_layers"].([]interface{}))
	}

	if len(f.Layers) == 0 {
		return nil, errors.New("remove_duplicate_features: must define source_layer XOR source_layers")
	}

	if c.Params["end_zoom"] != nil {
		f.EndZoom = float64(c.Params["end_zoom"].(int))
	}

	// required parameters
	f.Keys = parseStrings(c.Params["property_keys"])
	f.GeometryTypes = parseStrings(c.Params["geometry_types"])
	f.MinDistance = float64(c.Params["min_distance"].(float64))

	return f, nil
}

type dropFeaturesWhere struct {
	Layer     string
	StartZoom float64
	Condition filter.Condition
}

func (f *dropFeaturesWhere) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom {
		return
	}

	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	at := 0
	for _, feature := range layer.Features {
		ctx.fctx = filter.NewContextFromProperties(ctx.fctx, feature.Properties)
		if f.Condition.Eval(ctx.fctx) {
			continue
		}

		layer.Features[at] = feature
		at++
	}

	layer.Features = layer.Features[:at]
}

func compileDropFeaturesWhere(ctx *CompileContext, c *Config) (Function, error) {
	f := &dropFeaturesWhere{}

	f.Layer = c.Params["source_layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("drop_features_where: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	cond, err := filter.CompileCondition(c.Params["where"])
	if err != nil {
		return nil, err
	}

	f.Condition = cond
	return f, nil
}

// keyIsName returns true if this key looks like a name.
// This isn't as simple as testing if key == 'name', as there are alternative
// name-like tags such as 'official_name', translated names such as 'name:en',
// and left/right names for boundaries. This function aims to match all of
// those variants.
func keyIsName(key string) bool {
	// simplest and most common case first
	if key == "name" {
		return true
	}

	// translations next
	if strings.HasPrefix(key, "name:") {
		return true
	}

	// then any of the alternative forms of name
	tagNameAlternates := []string{
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

	for _, alt := range tagNameAlternates {
		if strings.HasPrefix(key, alt) {
			return true
		}
	}

	return false
}

func hasName(feature *geojson.Feature) bool {
	for k := range feature.Properties {
		if keyIsName(k) {
			return true
		}
	}

	return false
}

type dropNames struct {
	Layer     string
	StartZoom float64
	EndZoom   float64
}

func (f *dropNames) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom || f.EndZoom < ctx.Zoom {
		return
	}

	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	for _, feature := range layer.Features {
		for k := range feature.Properties {
			if keyIsName(k) {
				delete(feature.Properties, k)
			}
		}
	}
}

func compileDropNames(ctx *CompileContext, c *Config) (Function, error) {
	f := &dropNames{EndZoom: 50}

	f.Layer = c.Params["source_layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("drop_nams: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	ze, ok := c.Params["end_zoom"]
	if ok {
		z, ok := ze.(int)
		if !ok {
			return nil, errors.New("drop_names: end_zoom must be an integer")
		}

		f.EndZoom = float64(z)
	}

	return f, nil
}

// If a feature's name ends with a set of values in parens, update
// its kind and increase the min_zoom appropriately.
type updateParentheticalProperties struct {
	Layer         string
	TargetMinZoom float64
	DropBelowZoom float64
	Values        []string
	PValues       []string
}

func (f *updateParentheticalProperties) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	at := 0
features:
	for _, feature := range layer.Features {
		// check every property to see if it ends with the (*) type values

		for _, prop := range feature.Properties {
			for i, val := range f.PValues {
				if p, ok := prop.(string); ok && strings.HasSuffix(p, val) {
					feature.Properties["kind"] = f.Values[i]
					feature.Properties["min_zoom"] = f.TargetMinZoom
					if ctx.Zoom < f.DropBelowZoom {
						continue features
					}
				}
			}
		}

		layer.Features[at] = feature
		at++
	}

	layer.Features = layer.Features[:at]
}

func compileUpdateParentheticalProperties(ctx *CompileContext, c *Config) (Function, error) {
	f := &updateParentheticalProperties{}

	f.Layer = c.Params["source_layer"].(string)

	if _, ok := c.Params["start_zoom"]; ok {
		return nil, errors.New("update_parenthetical_properties: start_zoom not supported")
	}

	if _, ok := c.Params["end_zoom"]; ok {
		return nil, errors.New("update_parenthetical_properties: start_zoom not supported")
	}

	tz, ok := c.Params["target_min_zoom"]
	if ok {
		z, ok := tz.(int)
		if !ok {
			return nil, errors.New("update_parenthetical_properties: target zoom must be an integer")
		}

		f.TargetMinZoom = float64(z)
	}

	dz, ok := c.Params["drop_below_zoom"]
	if ok {
		z, ok := dz.(int)
		if !ok {
			return nil, errors.New("update_parenthetical_properties: drop_below_zoom must be an integer")
		}

		f.DropBelowZoom = float64(z)
	} else {
		return nil, errors.New("update_parenthetical_properties: drop_below_zoom is required")
	}

	v, ok := c.Params["values"]
	if ok {
		f.Values = parseStrings(v)
		for _, v := range f.Values {
			f.PValues = append(f.PValues, "("+v+")")
		}
	} else {
		return nil, errors.New("update_parenthetical_properties: values are required")
	}

	return f, nil
}

// Matches features from one layer with the other on the basis of the feature
// ID and, if the configured layer property doesn't exist on the feature, but
// the other layer property does exist on the matched feature, then copy it
// across.
// The initial use for this is to backfill POI kinds into building kind_detail
// when the building doesn't already have a different kind_detail supplied.
type backfillFromOtherLayers struct {
	SrcLayer string
	SrcKey   string
	DstLayer string
	DstKey   string
}

func (f *backfillFromOtherLayers) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	// build an index of feature ID to property value in the other layer
	values := make(map[int]interface{})
	for _, feature := range layers[f.SrcLayer].Features {
		fid := feature.Properties.MustInt("id", 0)
		if fid == 0 {
			continue
		}

		if kind, ok := feature.Properties[f.SrcKey]; ok {
			values[fid] = kind
		}
	}

	// apply those to features which don't already have a value
	for _, feature := range layers[f.DstLayer].Features {
		if _, ok := feature.Properties[f.DstKey]; ok {
			continue
		}

		id := feature.Properties.MustInt("id", 0)
		if id == 0 {
			continue
		}

		if v, ok := values[id]; ok {
			feature.Properties[f.DstKey] = v
		}
	}
}

func compileBackfillFromOtherLayers(ctx *CompileContext, c *Config) (Function, error) {
	f := &backfillFromOtherLayers{}

	var ok bool
	if f.SrcLayer, ok = c.Params["other_layer"].(string); !ok {
		return nil, errors.New("backfill_from_other_layer: other_layer must be defined")
	}

	if f.SrcKey, ok = c.Params["other_key"].(string); !ok {
		return nil, errors.New("backfill_from_other_layer: other_key must be defined")
	}

	if f.DstLayer, ok = c.Params["layer"].(string); !ok {
		return nil, errors.New("backfill_from_other_layer: layer must be defined")
	}

	if f.DstKey, ok = c.Params["layer_key"].(string); !ok {
		return nil, errors.New("backfill_from_other_layer: layer_key must be defined")
	}

	return f, nil
}

// ClipAndWrapGeometry clips the geometry in the layers, removing features that are
// clipped out. If possible it'll also wrap open polygon rings around the boundary
// so they look okay within the context of the boundary.
func ClipAndWrapGeometry(
	bound orb.Bound,
	clipFactors map[string]float64,
	layers map[string]*geojson.FeatureCollection,
) {
	// We clip open polygon (tainted multipolygon relations) to the tile boundary.
	// Clipping to larger area would result is correct geometry within the bound,
	// but invalid/overlapping geometry outside which tangram does not render correctly.
	// https://github.com/tangrams/tangram/issues/613
	//
	// Other geometry we clip to a +50% on each side bound. Since input for a tile
	// currently only includes ways with a node in the tile we need some overlap.
	for _, layer := range layers {
		paddedBound := padBoundByFactor(bound, 2.0)

		at := 0
		for _, f := range layer.Features {
			if hasOpenOuterRing(f.Geometry) {
				clipToBound(bound, f)
			} else {
				clipToBound(paddedBound, f)
			}

			if f.Geometry == nil {
				continue
			}

			layer.Features[at] = f
			at++
		}

		layer.Features = layer.Features[:at]
	}
}

func padBoundByFactor(b orb.Bound, f float64) orb.Bound {
	// padded/clipping bounds,
	//   1.0 (default) is same bounds.
	//   3.0 is 3x3 tile centered around tile.
	if f == 0 || f == 1.0 {
		return b
	}

	return geo.BoundPad(b, geo.BoundHeight(b)*(f-1)/2)
}

func clipToBound(b orb.Bound, f *geojson.Feature) {
	f.Geometry = smartclip.Geometry(b, f.Geometry, orb.CCW)
}

func hasOpenOuterRing(g orb.Geometry) bool {
	switch g := g.(type) {
	case orb.Polygon:
		return len(g) > 0 && !g[0].Closed()
	case orb.MultiPolygon:
		if len(g) == 0 {
			return false
		}

		for _, p := range g {
			if len(p) > 0 && !p[0].Closed() {
				return true
			}
		}
	}

	return false
}

func parseStrings(i interface{}) []string {
	vals := i.([]interface{})
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = v.(string)
	}

	return result
}

func stringIn(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}

	return false
}
