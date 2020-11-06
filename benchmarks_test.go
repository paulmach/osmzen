package osmzen

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/paulmach/osmzen/filter"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmapi"
	"github.com/paulmach/osm/osmgeojson"
)

func loadLayer(filename string) (*Layer, error) {
	l := &Layer{}
	err := l.load("layer", func(string) ([]byte, error) {
		return ioutil.ReadFile(filename)
	})

	return l, err
}

func BenchmarkBuildings(b *testing.B) {
	layer, err := loadLayer("config/yaml/buildings.yaml")
	if err != nil {
		b.Fatalf("failed to load: %v", err)
	}

	o := &osm.OSM{
		Ways: osm.Ways{
			{
				ID:      1,
				Version: 2,
				Nodes: osm.WayNodes{
					{ID: 1, Lat: 0, Lon: 0},
					{ID: 2, Lat: 0, Lon: 0.001},
					{ID: 3, Lat: 0.001, Lon: 0.001},
					{ID: 4, Lat: 0.001, Lon: 0},
					{ID: 1, Lat: 0, Lon: 0},
				},
				Tags: osm.Tags{
					{Key: "building", Value: "no"},
					{Key: "building:part", Value: "yes"},
				},
			},
		},
	}

	fc, err := osmgeojson.Convert(o,
		osmgeojson.NoID(true),
		osmgeojson.NoMeta(true),
		osmgeojson.NoRelationMembership(true),
	)
	if err != nil {
		b.Errorf("failed to convert to geojson: %v", err)
	}

	ctx := filter.NewContext(nil, fc.Features[0])

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkLayer(layer, ctx)
	}
}

func BenchmarkPOIs(b *testing.B) {
	layer, err := loadLayer("config/yaml/pois.yaml")
	if err != nil {
		b.Fatalf("failed to load: %v", err)
	}

	o := &osm.OSM{
		Nodes: osm.Nodes{
			{
				ID:      1,
				Version: 2,
				Tags: osm.Tags{
					{Key: "amenity", Value: "restaurant"},
					{Key: "cuisine", Value: "burger"},
					{Key: "name", Value: "Kronnerburger"},
				},
			},
		},
	}

	fc, err := osmgeojson.Convert(o,
		osmgeojson.NoID(true),
		osmgeojson.NoMeta(true),
		osmgeojson.NoRelationMembership(true),
	)
	if err != nil {
		b.Errorf("failed to convert to geojson: %v", err)
	}

	ctx := filter.NewContext(nil, fc.Features[0])

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkLayer(layer, ctx)
	}
}

// The matching benchmarks are to validate there are no allocations
// during layer matching.

func BenchmarkMatching_boundaries(b *testing.B) {
	matchLayer(b, "boundaries")
}

func BenchmarkMatching_buildings(b *testing.B) {
	matchLayer(b, "buildings")
}

func BenchmarkMatching_earth(b *testing.B) {
	matchLayer(b, "earth")
}

func BenchmarkMatching_landuse(b *testing.B) {
	matchLayer(b, "landuse")
}

func BenchmarkMatching_places(b *testing.B) {
	matchLayer(b, "places")
}

func BenchmarkMatching_pois(b *testing.B) {
	matchLayer(b, "pois")
}

func BenchmarkMatching_roads(b *testing.B) {
	matchLayer(b, "roads")
}

func BenchmarkMatching_transit(b *testing.B) {
	matchLayer(b, "transit")
}

func BenchmarkMatching_water(b *testing.B) {
	matchLayer(b, "water")
}

func matchLayer(b *testing.B, name string) {
	// Runtime for these benchmarks depend on the number of filters.
	// They are here to validate that there are no allocations.
	config, err := Load("config/queries.yaml")
	if err != nil {
		b.Fatalf("unable to load config: %v", err)
	}
	layer := config.Layers[name]

	o := &osm.OSM{
		Nodes: osm.Nodes{
			{
				ID:      1,
				Version: 2,
				Tags: osm.Tags{
					{Key: "amenity", Value: "restaurant"},
					{Key: "cuisine", Value: "burger"},
					{Key: "name", Value: "Kronnerburger"},
				},
			},
		},
	}

	fc, err := osmgeojson.Convert(o,
		osmgeojson.NoID(true),
		osmgeojson.NoMeta(true),
		osmgeojson.NoRelationMembership(true),
	)
	if err != nil {
		b.Errorf("failed to convert to geojson: %v", err)
	}

	ctx := filter.NewContext(nil, fc.Features[0])
	ctx.Debug = true

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, f := range layer.filters {
			f.Match(ctx)
		}
	}
}

func benchmarkLayer(layer *Layer, ctx *filter.Context) {
	ctx.Debug = true

	for _, f := range layer.filters {
		f.Match(ctx)

		// check the outputs
		for _, o := range f.Output {
			o.Expr.Eval(ctx)
		}
	}
}

func BenchmarkFullTile(b *testing.B) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		b.Fatalf("unable to load layer: %v", err)
	}

	tile := maptile.New(17896, 24450, 16)
	data := loadFile(b, tile)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := config.Process(data, tile.Bound(), tile.Z)
		if err != nil {
			b.Fatalf("procces failure: %v", err)
		}
	}
}

func BenchmarkProcessGeoJSON(b *testing.B) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		b.Fatalf("unable to load layer: %v", err)
	}

	tile := maptile.New(17896, 24450, 16)
	data := loadFile(b, tile)

	input, err := convertToGeoJSON(data, tile.Bound())
	if err != nil {
		b.Fatalf("convert failed: %v", err)
	}

	ctx := &zenContext{
		Zoom:  tile.Z,
		Bound: tile.Bound(),
		OSM:   data,
	}
	ctx.ComputeMembership()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := config.processGeoJSON(ctx, input, tile.Z)
		if err != nil {
			b.Fatalf("procces failure: %v", err)
		}
	}
}

func loadFile(t testing.TB, tile maptile.Tile) *osm.OSM {
	filename := fmt.Sprintf("testdata/tile-%d-%d-%d.xml", tile.Z, tile.X, tile.Y)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		bounds, err := osm.NewBoundsFromTile(tile)
		if err != nil {
			t.Fatalf("could not create bound: %v", err)
		}

		o, err := osmapi.Map(context.Background(), bounds)
		if err != nil {
			t.Fatalf("api call failed: %v", err)
		}

		data, _ := xml.MarshalIndent(o, "", " ")
		err = ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	} else if err != nil {
		t.Fatalf("file stat error: %v", err)
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	o := &osm.OSM{}
	err = xml.Unmarshal(data, &o)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	return o
}
