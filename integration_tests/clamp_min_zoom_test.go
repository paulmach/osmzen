package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osm"
)

func TestClampMinZoom(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 3}, {ID: 4}, {ID: 5}, {ID: 3},
			}, Tags: osm.Tags{
				{Key: "amenity", Value: "parking"},
				{Key: "building", Value: "yes"},
				{Key: "building:levels", Value: "7"},
				{Key: "name", Value: "parking garage"},
				{Key: "parking", Value: "multi-storey"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 3, Lat: 0.001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 4, Lat: 0.001, Lon: -0.001, Version: 1, Visible: true},
			{ID: 5, Lat: 0.000, Lon: -0.001, Version: 1, Visible: true},
		},
	}
	tile := processOSM(t, data)

	feature := tile["buildings"].Features[0]
	if gt := feature.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	if v := feature.Properties["scale_rank"]; v != 3.0 {
		// scale rank 3 should map to min_zoom 14
		t.Errorf("incorrect scale_rank: %v", v)
	}

	// from 13 -> 14 because of the camp for scale rank 3
	partialMatch(t, feature.Properties, geojson.Properties{
		"min_zoom": 14.0,
	})
}
