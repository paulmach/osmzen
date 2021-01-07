package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
)

func TestOnlyIncludeLabelIfInTile(t *testing.T) {
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
			{ID: 1, Lat: 1.0, Lon: 1.0, Version: 1, Visible: true},
			{ID: 2, Lat: 1.0, Lon: -0.00001, Version: 1, Visible: true},
			{ID: 3, Lat: -0.00001, Lon: -0.00001, Version: 1, Visible: true},
			{ID: 4, Lat: -0.00001, Lon: 1.0, Version: 1, Visible: true},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 15)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["landuse"].Features); l != 1 {
		t.Fatalf("should only have 1 landuse feature, the polygon")
	}

	if gt := tile["landuse"].Features[0].Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("landuse feature should be polygon: %v", gt)
	}

	if l := len(tile["pois"].Features); l != 0 {
		t.Errorf("should not include any pois")
	}
}

func TestIncludeLabelIfHouseName(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 2},
			}, Tags: osm.Tags{
				{Key: "name", Value: "my park"},
				{Key: "addr:housename", Value: "my house"},
				{Key: "building", Value: "yes"},
				{Key: "amenity", Value: "school"},
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
	x := uint32(1 << (16 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 16).Bound(), 16)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["buildings"].Features); l != 2 {
		t.Fatalf("should have building and label: %d", l)
	}

	building := tile["buildings"].Features[0]
	if n := building.Properties["name"]; n != "my house" {
		t.Errorf("building should not have house name: %v", n)
	}

	label := tile["buildings"].Features[1]
	if n := label.Properties["name"]; n != "my house" {
		t.Errorf("building should not have house name: %v", n)
	}

	// since there a "name" for the school, put it in the poi layer too.
	if l := len(tile["pois"].Features); l != 1 {
		t.Errorf("should have poi: %d", l)
	}
}
