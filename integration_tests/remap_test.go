package integrationtests

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestRemap(t *testing.T) {
	cases := []struct {
		name string
		tags osm.Tags
		kind string
	}{
		{
			name: "remapped",
			tags: osm.Tags{
				{Key: "military", Value: "airfield"},
				{Key: "area", Value: "yes"},
			},
			kind: "aerodrome",
		},
		{
			name: "not remapped",
			tags: osm.Tags{
				{Key: "leisure", Value: "dog_park"},
				{Key: "area", Value: "yes"},
			},
			kind: "dog_park",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := &osm.OSM{
				Ways: osm.Ways{
					{ID: 1, Visible: true, Nodes: osm.WayNodes{
						{ID: 3}, {ID: 4}, {ID: 5}, {ID: 3},
					}, Tags: tc.tags},
				},
				Nodes: osm.Nodes{
					{ID: 3, Lat: 0.1, Lon: 0.0000, Version: 1, Visible: true},
					{ID: 4, Lat: 0.1, Lon: -0.001, Version: 1, Visible: true},
					{ID: 5, Lat: 0.000, Lon: -0.001, Version: 1, Visible: true},
				},
			}

			// run the request
			tile := processOSM(t, data, 13)
			feature := tile["landuse"].Features[0]

			k := feature.Properties.MustString("kind")
			if k != tc.kind {
				t.Errorf("incorrect kind: %v != %v", k, tc.kind)
			}
		})
	}

	// not matching condition
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 3}, {ID: 4}, {ID: 5},
			}, Tags: osm.Tags{
				{Key: "waterway", Value: "dam"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 3, Lat: 0.1, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 4, Lat: 0.1, Lon: -0.001, Version: 1, Visible: true},
			{ID: 5, Lat: 0.000, Lon: -0.001, Version: 1, Visible: true},
		},
	}

	tile := processOSM(t, data, 13)
	feature := tile["landuse"].Features[0]

	// should not remap to barren
	k := feature.Properties.MustString("kind")
	if k != "dam" {
		t.Errorf("incorrect kind: %v", k)
	}
}
