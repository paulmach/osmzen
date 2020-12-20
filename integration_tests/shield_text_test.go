package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
)

func TestShieldText(t *testing.T) {
	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 2},
			}, Tags: osm.Tags{
				{Key: "highway", Value: "primary"},
				{Key: "name", Value: "West Superior Avenue"},
				{Key: "ref", Value: "US 6;US 20;US 42;SR 3"},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 1, Lat: 0.0000, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 2, Lat: 0.0001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 3, Lat: 0.0001, Lon: 0.0001, Version: 1, Visible: true},
			{ID: 4, Lat: 0.0000, Lon: 0.0001, Version: 1, Visible: true},
		},
		Relations: osm.Relations{
			{ID: 1, Version: 1, Visible: true, Members: osm.Members{{Type: "way", Ref: 1}},
				Tags: osm.Tags{
					{Key: "name", Value: "Ohio State Route 3"},
					{Key: "network", Value: "US:OH"},
					{Key: "ref", Value: "3"},
					{Key: "symbol", Value: "http://upload.wikimedia.org/wikipedia/commons/5/53/OH-3.svg"},
					{Key: "route", Value: "road"},
					{Key: "type", Value: "route"},
				},
			},
			{ID: 2, Version: 1, Visible: true, Members: osm.Members{{Type: "way", Ref: 1}},
				Tags: osm.Tags{
					{Key: "name", Value: "US 6 (OH)"},
					{Key: "network", Value: "US:US"},
					{Key: "ref", Value: "6"},
					{Key: "route", Value: "road"},
					{Key: "type", Value: "route"},
				},
			},
			{ID: 3, Version: 1, Visible: true, Members: osm.Members{{Type: "way", Ref: 1}},
				Tags: osm.Tags{
					{Key: "name", Value: "US 20 (OH)"},
					{Key: "network", Value: "US:US"},
					{Key: "ref", Value: "20"},
					{Key: "symbol", Value: "http://upload.wikimedia.org/wikipedia/commons/9/92/US_20.svg"},
					{Key: "route", Value: "road"},
					{Key: "type", Value: "route"},
				},
			},
			{ID: 4, Version: 1, Visible: true, Members: osm.Members{{Type: "way", Ref: 1}},
				Tags: osm.Tags{
					{Key: "name", Value: "US 42 (OH)"},
					{Key: "network", Value: "US:US"},
					{Key: "ref", Value: "42"},
					{Key: "route", Value: "road"},
					{Key: "type", Value: "route"},
				},
			},
		},
	}

	// run the request
	tile, err := config.Process(data, maptile.Tile{}.Bound(), 20)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if v := tile["roads"].Features[0].Properties["shield_text"]; v != "6" {
		t.Errorf("incorrect shield text: %v", v)
	}
}
