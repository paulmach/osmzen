package integrationtests

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestCollisionRank(t *testing.T) {
	cases := []struct {
		name string
		tags osm.Tags
		rank int
	}{
		{
			name: "beach",
			tags: osm.Tags{
				{Key: "natural", Value: "beach"},
				{Key: "name", Value: "Stinson Beach"},
			},
			rank: 534,
		},
		{
			name: "population",
			tags: osm.Tags{
				{Key: "name", Value: "Berkeley"},
				{Key: "population", Value: "120000"},
				{Key: "place", Value: "city"},
			},
			rank: 350,
		},
		{
			name: "population",
			tags: osm.Tags{
				{Key: "name", Value: "Berkeley"},
				{Key: "population", Value: "210000"},
				{Key: "place", Value: "city"},
			},
			rank: 347,
		},
		{
			name: "building exit",
			tags: osm.Tags{
				{Key: "name", Value: "exit"},
				{Key: "entrance", Value: "fire_exit"},
			},
			rank: 4303,
		},
		{
			name: "no rank if no name",
			tags: osm.Tags{
				{Key: "building", Value: "true"},
			},
			rank: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := &osm.OSM{
				Nodes: osm.Nodes{{ID: 1, Visible: true, Version: 1, Tags: tc.tags}},
			}

			// run the request
			tile := processOSM(t, data)

			for name, layer := range tile {
				for _, feature := range layer.Features {
					r := feature.Properties.MustInt("collision_rank", 0)
					if r != tc.rank {
						t.Errorf("layer %v: incorrect rank: %v != %v", name, r, tc.rank)
					}
				}
			}
		})
	}
}
