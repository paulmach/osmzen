package osmzen

import (
	"math"

	"github.com/paulmach/osmzen/filter"
	"github.com/paulmach/osmzen/postprocess"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"

	"github.com/pkg/errors"
)

// Process will convert OSM data into geojson layers.
// The bound is used for clipping large geometry and only returning label "points"
// if they're in the bound.  The zoom is used to do the correct post process filtering.
func (c *Config) Process(data *osm.OSM, bound orb.Bound, z maptile.Zoom) (map[string]*geojson.FeatureCollection, error) {
	return c.process(data, bound, z)
}

// order is the prefered order to process a single element.
// Try to match the most reasonable first.
var order = []string{
	"pois",
	"roads",
	"buildings",
	"landuse",
	"water",
	"places",

	"boundaries",
	"transit",
	"earth",
}

// ProcessElement will convert a single osm element to [layer, properties].
// It will return the first matching layer.
func (c *Config) ProcessElement(e osm.Element) (layer string, props geojson.Properties, err error) {
	data := &osm.OSM{}
	data.Append(e)

	layers, err := c.process(data, orb.Bound{Min: orb.Point{-180, -90}, Max: orb.Point{180, 90}}, 20)
	if err != nil {
		return "", nil, err
	}

	for _, o := range order {
		if layer := layers[o]; layer != nil {
			return o, layer.Features[0].Properties, nil
		}
	}

	// an extra check in case the list of layers does not
	// match the `order` list above.
	for _, o := range c.All {
		if !stringIn(o, order) {
			if layer := layers[o]; layer != nil {
				return o, layer.Features[0].Properties, nil
			}
		}
	}

	return "", nil, errors.New("not found")
}

func (c *Config) process(data *osm.OSM, bound orb.Bound, z maptile.Zoom) (map[string]*geojson.FeatureCollection, error) {
	input, err := convertToGeoJSON(data, bound)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx := &zenContext{
		Zoom:  z,
		Bound: bound,
		OSM:   data,
	}
	ctx.ComputeMembership()

	return c.processGeoJSON(ctx, input, z)
}

func (c *Config) processGeoJSON(
	ctx *zenContext,
	input *geojson.FeatureCollection,
	z maptile.Zoom,
) (map[string]*geojson.FeatureCollection, error) {
	result := make(map[string]*geojson.FeatureCollection, len(c.Layers))
	for _, name := range c.All {
		lc, ok := c.Layers[name]
		if !ok {
			return nil, errors.Errorf("layer not defined: %v", name)
		}

		f, err := lc.evalFeatures(ctx, input)
		if err != nil {
			return nil, err
		}
		result[name] = f
	}

	// apply post processing
	ppctx := &postprocess.Context{
		Zoom:  float64(z),
		Bound: ctx.Bound,
	}

	// This does some "what is the name really" logic that is part
	// of the initial SQL query in the tilezen/vector-datasource.
	postprocess.SetConditionalNames(ppctx, result)

	for _, pp := range c.postProcessors {
		pp.Eval(ppctx, result)
	}

	// We clip exactly to the tile boundary because that is the only area
	// where we know we have complete geometry. Clipping to larger area would
	// result is correct geometry within the bound, but invalid/overlapping geometry
	// outside which tangram does not render correctly.
	// https://github.com/tangrams/tangram/issues/613
	//
	// Also clipping to exactly the tile boundary may cause problems with showing
	// outlines along the tile boundary.
	// https://github.com/tilezen/vector-datasource/issues/197
	// postprocess.ClipAndWrapGeometry(ppctx.Bound, result)

	// remove tags
	for _, l := range result {
		for _, f := range l.Features {
			delete(f.Properties, "tags")
		}
	}

	return result, nil
}

// Process will convert OSM data into a feature collection for that layer.
// The zoom is used to do the correct post process filtering.
func (l *Layer) Process(data *osm.OSM, bound orb.Bound, z maptile.Zoom) (*geojson.FeatureCollection, error) {
	input, err := convertToGeoJSON(data, bound)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx := &zenContext{
		Zoom:  z,
		Bound: bound,
		OSM:   data,
	}
	ctx.ComputeMembership()

	return l.evalFeatures(ctx, input)
}

func (l *Layer) evalFeatures(
	ctx *zenContext,
	input *geojson.FeatureCollection,
) (*geojson.FeatureCollection, error) {
	output := geojson.NewFeatureCollection()
	for _, f := range input.Features {
		// ways that intersect the tile many have interesting nodes outside the tile.
		// these nodes become geojson points that we want to skip.
		if p, ok := f.Geometry.(orb.Point); ok {
			if !ctx.Bound.Contains(p) {
				continue
			}
		}

		feature, err := l.evalFeature(ctx, f)
		if err != nil {
			return nil, err
		}

		if feature == nil {
			continue // no match
		}

		// big polygons may have pois that are not in the tile.
		if p, ok := feature.Geometry.(orb.Point); ok {
			if !ctx.Bound.Contains(p) {
				continue
			}
		}

		output.Append(feature)
	}

	return output, nil
}

func (l *Layer) evalFeature(ctx *zenContext, feature *geojson.Feature) (*geojson.Feature, error) {
	fctx := filter.NewContext(feature)

	fctx.OSM = ctx.OSM
	fctx.WayMembership = ctx.WayMembership
	fctx.RelationMembership = ctx.RelationMembership

	if !stringIn(fctx.Geometry.GeoJSONType(), l.GeometryTypes) {
		return nil, nil
	}

	result, err := l.filterMatch(fctx)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	if result.MinZoom == nil {
		// skip this feature, see pois.yaml for an example
		return nil, nil
	}

	minZoom := result.MinZoom.EvalNum(fctx)

	// zoom 12 tile, return all features with [0, 13) min_zoom
	// zoom 12 tile, do not return min_zoom features [13 inf)
	if float64(ctx.Zoom+1) < minZoom {
		return nil, nil
	}

	output := geojson.NewFeature(fctx.Geometry)
	output.Properties = result.Properties(fctx)
	output.Properties["min_zoom"] = math.Floor(minZoom*100) / 100.0

	// tilezen/vector-datasource has relations have negative ids, not sure why exactly.
	if feature.Properties.MustString("type") == "relation" {
		output.Properties["id"] = -feature.Properties.MustInt("id")
	} else {
		output.Properties["id"] = feature.Properties.MustInt("id")
	}
	output.Properties["type"] = string(feature.Properties.MustString("type"))

	// original element tags used a few places as part of post processing.
	output.Properties["tags"] = feature.Properties["tags"]

	l.applyTransforms(fctx, output)
	return output, nil
}

func (l *Layer) applyTransforms(fctx *filter.Context, feature *geojson.Feature) {
	for _, transform := range l.transforms {
		transform(fctx, feature)
	}
}

func (l *Layer) filterMatch(fctx *filter.Context) (*filter.Filter, error) {
	for _, f := range l.filters {
		if f.Match(fctx) {
			return f, nil
		}
	}

	return nil, nil
}

type zenContext struct {
	Zoom               maptile.Zoom
	Bound              orb.Bound
	OSM                *osm.OSM
	WayMembership      map[osm.NodeID]osm.Ways
	RelationMembership map[osm.FeatureID]osm.Relations
}

func (ctx *zenContext) ComputeMembership() {
	if ctx.OSM == nil {
		return
	}

	// TODO: I wish you could pass all this stuff in.
	// it's kind of messy as is.
	nodes := make(map[osm.NodeID]*osm.Node, len(ctx.OSM.Nodes))
	for _, n := range ctx.OSM.Nodes {
		nodes[n.ID] = n
	}

	ctx.WayMembership = make(map[osm.NodeID]osm.Ways)
	for _, w := range ctx.OSM.Ways {
		for _, wn := range w.Nodes {
			if n, ok := nodes[wn.ID]; ok && len(n.Tags) == 0 {
				continue
			}
			ctx.WayMembership[wn.ID] = append(ctx.WayMembership[wn.ID], w)
		}
	}

	ctx.RelationMembership = make(map[osm.FeatureID]osm.Relations)
	for _, r := range ctx.OSM.Relations {
		for _, m := range r.Members {
			ctx.RelationMembership[m.FeatureID()] = append(ctx.RelationMembership[m.FeatureID()], r)
		}
	}
}

func convertToGeoJSON(data *osm.OSM, bound orb.Bound) (*geojson.FeatureCollection, error) {
	fc, err := osmgeojson.Convert(data,
		osmgeojson.IncludeInvalidPolygons(true),
		osmgeojson.NoID(true),
		osmgeojson.NoMeta(true),
		osmgeojson.NoRelationMembership(true),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// The osmgeojson.IncludeInvalidPolygons option will allow us to get
	// polygons with open outer rings or even completely missing outer rings
	// if just the inners intersect the bounds. The missing outer rings need to be
	// replaced with the bound. Open outer rings will "cropped and wrapped" towards
	// the end of the whole process.
	padded := geo.BoundPad(bound, geo.BoundWidth(bound))
	for _, f := range fc.Features {
		switch g := f.Geometry.(type) {
		case orb.MultiPolygon:
			for _, p := range g {
				if len(p) > 0 && p[0] == nil {
					p[0] = padded.ToRing()
				}
			}
		case orb.Polygon:
			if len(g) > 0 && g[0] == nil {
				g[0] = padded.ToRing()
			}
		}
	}

	return fc, nil
}

func stringIn(val string, list []string) bool {
	for _, l := range list {
		if l == val {
			return true
		}
	}

	return false
}
