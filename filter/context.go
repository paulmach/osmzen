package filter

import (
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/orb/project"
	"github.com/paulmach/osm"
)

// Context is the evaluation context which is a bag of data.
// Also used as a data cache.
type Context struct {
	Debug   bool
	Verbose bool

	FeatureID osm.FeatureID
	Geometry  orb.Geometry

	// To compute ways and/or relations if needed
	OSM *osm.OSM

	Tags map[string]string

	// OSMTags are used during post processing we need access
	// the original osm tags as well as the new filter outputs.
	OSMTags map[string]string

	// cached data
	length  float64
	area    float64
	minZoom float64

	ways      osm.Ways
	relations osm.Relations

	WayMembership      map[osm.NodeID]osm.Ways
	RelationMembership map[osm.FeatureID]osm.Relations
}

// NewContext creates a new filter.Context from an osmgeojson feature.
func NewContext(feature *geojson.Feature) *Context {
	ctx := &Context{
		length:  -1,
		area:    -1,
		minZoom: -1,
	}

	if feature != nil {
		ctx.FeatureID = osm.Type(feature.Properties.MustString("type")).
			FeatureID(int64(feature.Properties.MustInt("id")))

		// nil feature are sometimes passed in when testing.
		// not setting one or both of these values will break lots of things.
		ctx.Geometry = feature.Geometry
		ctx.Tags = feature.Properties["tags"].(map[string]string)
	}

	return ctx
}

// NewContextFromProperties will create a context using a set of properies.
// This limits the queries one can do since not all the geometry is present.
func NewContextFromProperties(props geojson.Properties) *Context {
	tags := make(map[string]string)
	for k, v := range props {
		if s, ok := v.(string); ok {
			tags[k] = s
		}
	}

	osmTags, _ := props["tags"].(map[string]string)
	ctx := &Context{
		Tags:    tags,
		OSMTags: osmTags,
		length:  -1,
		area:    -1,
		minZoom: -1,
	}

	ctx.FeatureID = osm.Type(props.MustString("type")).FeatureID(int64(props.MustInt("id")))

	return ctx
}

// Length computes the length of the element. This is used many
// places so it's computed/cacched on the context.
func (ctx *Context) Length() float64 {
	if ctx.length >= 0 {
		return ctx.length
	}

	ctx.computeLengthArea()
	return ctx.length
}

// Area computes the area of the element. This is used many
// places so it's computed/cacched on the context.
func (ctx *Context) Area() float64 {
	if ctx.area >= 0 {
		return ctx.area
	}

	ctx.computeLengthArea()
	return ctx.area
}

func (ctx *Context) computeLengthArea() {
	projected := project.ToPlanar(orb.Clone(ctx.Geometry), project.Mercator)

	ctx.area = planar.Area(projected)
	ctx.area = math.Floor(ctx.area + 0.5)

	switch g := projected.(type) {
	case orb.LineString:
		ctx.length = planar.Length(g)
	case orb.Polygon:
		ctx.length = planar.Length(g[0])
	default:
		ctx.length = 0
	}
}

// Height returns the height of the thing, usually a building.
func (ctx *Context) Height() float64 {
	return math.Floor(buildingHeight(ctx) + 0.5)
}

var log4 = math.Log(4)

// MinZoom computes the min zoom that the attributes should be displayed at.
// https://github.com/tilezen/vector-datasource/blob/e01d363c7279ba23c2e61c26b3562dccb7f33e60/data/functions.sql#L166-L187
func (ctx *Context) MinZoom() float64 {
	if ctx.minZoom >= 0 {
		// we really just cache this so we can stub for tests.
		return ctx.minZoom
	}

	// returns the (possibly fractional) zoom at which the given way
	// area will be one square pixel nominally on screen (assuming
	// that tiles are 256x256px at integer zooms). sadly, features
	// aren't always rectangular and axis-aligned, but this should
	// still give a reasonable approximation to the zoom that it
	// would be appropriate to show them.

	// can't take logarithm of zero, and some ways have
	// incredibly tiny areas, down to even zero. also, by z16
	// all features really should be visible, so we clamp the
	// computation at the way area which would result in 16
	// being returned.
	area := ctx.Area()
	if area < 5.704 {
		return 16.0
	}

	z := 17.256 - math.Log(area)/log4
	ctx.minZoom = math.Floor(z*100) / 100

	return ctx.minZoom
}

func (ctx *Context) wayMembership() osm.Ways {
	if ctx.FeatureID.Type() != osm.TypeNode {
		return nil
	}

	if ctx.WayMembership != nil {
		return ctx.WayMembership[ctx.FeatureID.NodeID()]
	}

	return ctx.computeWays()
}

func (ctx *Context) computeWays() osm.Ways {
	if ctx.OSM == nil {
		return nil
	}

	if ctx.ways != nil {
		return ctx.ways // already computed
	}

	ctx.ways = make(osm.Ways, 0, 5)
	for _, w := range ctx.OSM.Ways {
		for _, n := range w.Nodes {
			if n.FeatureID() == ctx.FeatureID {
				ctx.ways = append(ctx.ways, w)
				break
			}
		}
	}

	return ctx.ways
}

// relationMembership returns the relations the element is a member of.
// It can use the RelationMembership map if provided. This is useful
// if evaluating many elements. Otherwise it computes it on demand
// from the ctx.OSM if provided. If that also doesn't exist, returns nil/empty.
func (ctx *Context) relationMembership() osm.Relations {
	if ctx.RelationMembership != nil {
		return ctx.RelationMembership[ctx.FeatureID]
	}

	return ctx.computeRelations()
}

func (ctx *Context) computeRelations() osm.Relations {
	if ctx.OSM == nil {
		return nil
	}

	if ctx.relations != nil {
		return ctx.relations // already computed
	}

	ctx.relations = make(osm.Relations, 0, 5)
	for _, r := range ctx.OSM.Relations {
		for _, m := range r.Members {
			if m.FeatureID() == ctx.FeatureID {
				ctx.relations = append(ctx.relations, r)
				break
			}
		}
	}

	return ctx.relations
}
