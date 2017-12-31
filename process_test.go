package osmzen

import (
	"encoding/xml"
	"reflect"
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
)

func TestProcess(t *testing.T) {
	// This test goes through the motions and converts a geojson building.
	data := `<osm>
 <way id="22942652" user="andygol" uid="94578" visible="true" version="6" changeset="46172437" timestamp="2017-02-17T18:21:11Z">
  <nd ref="247139891" version="3" changeset="16969271" lat="37.8243324" lon="-122.2565497"></nd>
  <nd ref="247139892" version="3" changeset="16969271" lat="37.8249618" lon="-122.2557092"></nd>
  <nd ref="247139893" version="3" changeset="16969271" lat="37.8244875" lon="-122.2551399"></nd>
  <nd ref="247139894" version="3" changeset="16969271" lat="37.8238958" lon="-122.25593"></nd>
  <nd ref="247139895" version="3" changeset="16969271" lat="37.8241277" lon="-122.2562084"></nd>
  <nd ref="247139896" version="3" changeset="16969271" lat="37.82409" lon="-122.2562588"></nd>
  <nd ref="247139891" version="3" changeset="16969271" lat="37.8243324" lon="-122.2565497"></nd>
  <tag k="amenity" v="parking"></tag>
  <tag k="building" v="yes"></tag>
  <tag k="building:levels" v="7"></tag>
  <tag k="name" v="Kaiser Permanente Medical Center - Parking Garage"></tag>
  <tag k="parking" v="multi-storey"></tag>
 </way>
</osm>`

	o := &osm.OSM{}
	err := xml.Unmarshal([]byte(data), &o)
	if err != nil {
		t.Fatalf("unable to unmarshal xml: %v", err)
	}

	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

	tile, err := config.Process(o, maptile.Tile{}.Bound(), 20)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	feature := tile["buildings"].Features[0]
	if gt := feature.Geometry.GeoJSONType(); gt != geojson.Polygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.Polygon)
	}

	expected := map[string]interface{}{
		"min_zoom":    13.0,
		"sort_rank":   475.0,
		"scale_rank":  2.0,
		"height":      23.0,
		"area":        11528.0,
		"volume":      265144.0,
		"kind":        "building",
		"kind_detail": "parking",
		"id":          22942652,
		"type":        "way",
	}

	if !reflect.DeepEqual(feature.Properties, geojson.Properties(expected)) {
		t.Errorf("incorrect properties: %v", feature.Properties)
	}
}

func TestProcessTile_OnlyPOISInTile(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

	data := &osm.OSM{
		Ways: osm.Ways{
			{ID: 1, Visible: true, Nodes: osm.WayNodes{
				{ID: 1}, {ID: 2},
			}},
		},
		Nodes: osm.Nodes{
			{ID: 1, Lat: 0.00001, Lon: 0.00001, Version: 1, Visible: true},
			{ID: 2, Lat: -0.00001, Lon: -0.00001, Version: 1, Visible: true,
				Tags: osm.Tags{
					{Key: "name", Value: "my park"},
					{Key: "leisure", Value: "park"},
				}},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 15)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["pois"].Features); l != 0 {
		t.Errorf("should not include any pois")
	}
}

func TestProcessTile_OnlyIncludeLabelIfInTile(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

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

	if gt := tile["landuse"].Features[0].Geometry.GeoJSONType(); gt != geojson.Polygon {
		t.Errorf("landuse feature should be polygon: %v", gt)
	}

	if l := len(tile["pois"].Features); l != 0 {
		t.Errorf("should not include any pois")
	}
}

func TestProcessTile_DeduplicatePOIS(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

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
			{ID: 1, Lat: 0.0000, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 2, Lat: 0.0001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 3, Lat: 0.0001, Lon: 0.0001, Version: 1, Visible: true},
			{ID: 4, Lat: 0.0000, Lon: 0.0001, Version: 1, Visible: true},
		},
	}

	// run the request
	x := uint32(1 << (15 - 1))
	tile, err := config.Process(data, maptile.New(x, x-1, 15).Bound(), 16)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	if l := len(tile["landuse"].Features); l != 1 {
		t.Fatalf("should only have 1 landuse feature, the polygon: %d", l)
	}

	if gt := tile["landuse"].Features[0].Geometry.GeoJSONType(); gt != geojson.Polygon {
		t.Errorf("landuse feature should be polygon: %v", gt)
	}

	if l := len(tile["pois"].Features); l != 1 {
		t.Errorf("should include poi for the part")
	}
}

func TestProcessTile_SchoolBuildingInOneLayer(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

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
			{ID: 1, Lat: 0.0000, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 2, Lat: 0.0001, Lon: 0.0000, Version: 1, Visible: true},
			{ID: 3, Lat: 0.0001, Lon: 0.0001, Version: 1, Visible: true},
			{ID: 4, Lat: 0.0000, Lon: 0.0001, Version: 1, Visible: true},
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

func TestProcessTile_IncludeLabelIfHouseName(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

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

func TestProcessTile_Height(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

	cases := []struct {
		name   string
		tags   osm.Tags
		height float64
	}{
		{
			name: "building height",
			tags: osm.Tags{
				{Key: "height", Value: "10"},
				{Key: "building", Value: "yes"},
			},
			height: 10,
		},
		{
			name: "building levels",
			tags: osm.Tags{
				{Key: "building:levels", Value: "7"},
				{Key: "building", Value: "yes"},
			},
			height: 23,
		},
		{
			name: "waterfall height",
			tags: osm.Tags{
				{Key: "height", Value: "4"},
				{Key: "waterway", Value: "waterfall"},
				{Key: "name", Value: "Great Falls of Tinker's Creek"},
			},
			height: 4,
		},
		{
			name: "waterfall height in meters",
			tags: osm.Tags{
				{Key: "height", Value: "4m"},
				{Key: "waterway", Value: "waterfall"},
				{Key: "name", Value: "Great Falls of Tinker's Creek"},
			},
			height: 4,
		},
		{
			name: "waterfall height in feet",
			tags: osm.Tags{
				{Key: "height", Value: "10ft"},
				{Key: "waterway", Value: "waterfall"},
				{Key: "name", Value: "Great Falls of Tinker's Creek"},
			},
			height: 3.048,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := &osm.OSM{
				Nodes: osm.Nodes{{ID: 1, Visible: true, Version: 1, Tags: tc.tags}},
			}

			// run the request
			tile, err := config.Process(data, maptile.Tile{}.Bound(), 20)
			if err != nil {
				t.Fatalf("unable to build geojson: %v", err)
			}

			for _, layer := range tile {
				for _, feature := range layer.Features {
					if feature.Properties.MustInt("id") == 1 {
						h := feature.Properties.MustFloat64("height")
						if h != tc.height {
							t.Errorf("incorrect height: %v != %v", h, tc.height)
						}
					}
				}
			}
		})
	}
}

func TestProcessTile_ShieldText(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		t.Fatalf("unable to load layer: %v", err)
	}

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
