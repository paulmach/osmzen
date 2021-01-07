package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
)

func TestSchoolBuildingInOneLayer(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 2},
			}, Tags: osm.Tags{
				{Key: "name", Value: "my park"},
				{Key: "building", Value: "yes"},
				{Key: "amenity", Value: "school"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 2, Lat: 0.0000, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 3, Lat: 0.0001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 4, Lat: 0.0001, Lon: 0.0001, Version: 1, Visible: true},
			{ID: 5, Lat: 0.0000, Lon: 0.0001, Version: 1, Visible: true},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 16)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["buildings"].Features); l != 1 {
		t.Fatalf("should have building: %d", l)
	}

	building := tile["buildings"].Features[0]
	if n, ok := building.Properties["name"]; ok {
		t.Errorf("building should not have name: %v", n)
	}

	if l := len(tile["landuse"].Features); l != 0 {
		t.Fatalf("should be left out of landuse layer: %d", l)
	}

	if l := len(tile["pois"].Features); l != 1 {
		t.Errorf("should have poi: %d", l)
	}
}
