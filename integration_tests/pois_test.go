package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
)

func TestOnlyPOISInTile(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 1}, {ID: 2},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 1, Lat: 0.00001, Lon: 0.00001, Version: 1, Visible: true},
			{ID: 2, Lat: -0.00001, Lon: -0.00001, Version: 1, Visible: true,
				Tags: osm.Tags{
					{Key: "name", Value: "my park"},
					{Key: "leisure", Value: "park"},
				}},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 15)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["pois"].Features); l != 0 {
		t.Errorf("should not include any pois")
	}
}

func TestDeduplicatePOIS(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 1},
			}, Tags: osm.Tags{
				{Key: "name", Value: "my park"},
				{Key: "leisure", Value: "park"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 1, Lat: 0.0000, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 2, Lat: 0.0001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 3, Lat: 0.0001, Lon: 0.0001, Version: 1, Visible: true},
			{ID: 4, Lat: 0.0000, Lon: 0.0001, Version: 1, Visible: true},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 16)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["landuse"].Features); l != 1 {
		t.Fatalf("should only have 1 landuse feature, the polygon: %d", l)
	}

	if gt := tile["landuse"].Features[0].Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("landuse feature should be polygon: %v", gt)
	}

	if l := len(tile["pois"].Features); l != 1 {
		t.Errorf("should include poi for the part")
	}
}
