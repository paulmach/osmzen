package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osm"
)

func TestDropNames(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 3}, {ID: 4}, {ID: 5}, {ID: 3},
			}, Tags: osm.Tags{
				{Key: "area", Value: "yes"},
				{Key: "zoo", Value: "petting_zoo"},
				{Key: "surface", Value: "dirt"},
				{Key: "name", Value: "osm zoo"},
				{Key: "name:en", Value: "osm zoo"},
				{Key: "old_name:en", Value: "osm zoo"},
				{Key: "short_name", Value: "osmz"},
				{Key: "name:short", Value: "osmz"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 3, Lat: 0.001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 4, Lat: 0.001, Lon: -0.001, Version: 1, Visible: true},
			{ID: 5, Lat: 0.000, Lon: -0.001, Version: 1, Visible: true},
		},
	}

	// zoom level 13 should remove all the 'name like' keys.
	tile := processOSM(t, data, 13)

	feature := tile["landuse"].Features[0]
	if gt := feature.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	missingKeys := []string{
		"name", "name:en",
		"old_name:en", "short_name",
		"name:short",
	}
	for _, key := range missingKeys {
		if v, ok := feature.Properties[key]; ok {
			t.Errorf("key '%s' should be missing: %v", key, v)
		}
	}
}
