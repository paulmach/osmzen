package ranker

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestRanker(t *testing.T) {
	ranker, err := LoadFile("../config/spreadsheets/collision_rank.yaml")
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	cases := []struct {
		name  string
		layer string
		props geojson.Properties
		rank  int
	}{
		{
			name:  "first",
			layer: "earth",
			props: geojson.Properties{
				"kind": "continent",
			},
			rank: 300,
		},
		{
			name:  "one property",
			layer: "places",
			props: geojson.Properties{
				"kind":            "country",
				"population_rank": 12,
			},
			rank: 308,
		},

		{
			name:  "multiple properties 1",
			layer: "places",
			props: geojson.Properties{
				"kind":            "locality",
				"population_rank": 18,
				"country_capital": true,
			},
			rank: 321,
		},
		{
			name:  "multiple properties 2",
			layer: "places",
			props: geojson.Properties{
				"kind":            "locality",
				"population_rank": 18,
				"region_capital":  true,
			},
			rank: 322,
		},
		{
			name:  "multiple properties 3",
			layer: "places",
			props: geojson.Properties{
				"kind":            "locality",
				"population_rank": 18,
			},
			rank: 323,
		},

		// diff should be 20 because of _reserved/count
		{
			name:  "multiple properties",
			layer: "roads",
			props: geojson.Properties{
				"kind":        "major_road",
				"kind_detail": "secondary",
			},
			rank: 599,
		},
		{
			name:  "multiple properties",
			layer: "roads",
			props: geojson.Properties{
				"kind":        "major_road",
				"kind_detail": "secondary_link",
			},
			rank: 620,
		},

		// not
		{
			name:  "matching boundaries",
			layer: "boundaries",
			props: geojson.Properties{
				"kind": "country",
			},
			rank: 807,
		},
		{
			name:  "matching boundaries",
			layer: "boundaries",
			props: geojson.Properties{
				"kind":              "country",
				"maritime_boundary": true,
			},
			rank: 2375,
		},

		// catchall
		{
			name:  "catchall",
			layer: "pois",
			props: geojson.Properties{
				"kind": "country",
				"name": "abc",
			},
			rank: 4324,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rank := ranker.Rank(tc.layer, tc.props)
			if rank != tc.rank {
				t.Errorf("incorrect rank: %v != %v", rank, tc.rank)
			}
		})
	}
}
