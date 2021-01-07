package integrationtests

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestUpdateParentheticalProperties(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 3}, {ID: 4}, {ID: 5}, {ID: 3},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 3, Lat: 0.001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 4, Lat: 0.001, Lon: -0.01, Version: 1, Visible: true},
			{ID: 5, Lat: 0.000, Lon: -0.01, Version: 1, Visible: true},
		},
	}

	cases := []struct {
		name string
		tags osm.Tags
		kind string
	}{
		{
			name: "closed building",
			tags: osm.Tags{
				{Key: "building", Value: "yes"},
				{Key: "name", Value: "abc (closed)"},
			},
			kind: "closed",
		},
		{
			name: "historical building",
			tags: osm.Tags{
				{Key: "building", Value: "yes"},
				{Key: "name", Value: "abc (historical)"},
			},
			kind: "historical",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data.Ways[0].Tags = tc.tags

			// run the request
			tile := processOSM(t, data, 16)
			feature := tile["buildings"].Features[0]
			if v := feature.Properties["kind"]; v != tc.kind {
				t.Errorf("incorrect kind: %v != %v", v, tc.kind)
			}

			// run at zoom 15, should cause features to be dropped
			tile = processOSM(t, data, 15)
			if l := len(tile["buildings"].Features); l != 0 {
				t.Errorf("should skip item: %v", tile["buildings"].Features[0])
			}
		})
	}
}
