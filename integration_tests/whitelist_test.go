package integrationtests

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestWhitelist(t *testing.T) {
	cases := []struct {
		name    string
		tags    osm.Tags
		surface string
	}{
		{
			name: "allowed",
			tags: osm.Tags{
				{Key: "highway", Value: "living_street"},
				{Key: "surface", Value: "paved"},
			},
			surface: "paved",
		},
		{
			name: "remapped",
			tags: osm.Tags{
				{Key: "highway", Value: "living_street"},
				{Key: "surface", Value: "clay"},
			},
			surface: "unpaved",
		},
		{
			name: "remove",
			tags: osm.Tags{
				{Key: "highway", Value: "living_street"},
				{Key: "surface", Value: "not in whitelist or remap"},
			},
			surface: "",
		},
		{
			name: "not matching condition",
			tags: osm.Tags{
				{Key: "highway", Value: "pedestrian"},
				{Key: "surface", Value: "random"},
			},
			surface: "random",
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
					{ID: 3, Lat: 0.001, Lon: 0.0000, Version: 1, Visible: true},
					{ID: 4, Lat: 0.001, Lon: -0.001, Version: 1, Visible: true},
					{ID: 5, Lat: 0.000, Lon: -0.001, Version: 1, Visible: true},
				},
			}

			// run the request
			tile := processOSM(t, data, 15)
			feature := tile["roads"].Features[0]

			v, ok := feature.Properties["surface"]
			if !ok {
				if tc.surface != "" {
					t.Errorf("surface should have value")
				}
				return
			}

			s := v.(string)
			if s != tc.surface {
				t.Errorf("incorrect surface: %v != %v", v, tc.surface)
			}
		})
	}
}
