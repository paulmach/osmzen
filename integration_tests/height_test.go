package integrationtests

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestHeight(t *testing.T) {
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
			tile := processOSM(t, data)

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

func TestQuantizeHeight(t *testing.T) {
	data := []byte(`
		<osm>
		 <way id="22942652" visible="true" version="6">
		  <nd ref="2" version="3" lat="0.001" lon="0"></nd>
		  <nd ref="3" version="3" lat="0.001" lon="-0.001"></nd>
		  <nd ref="4" version="3" lat="0" lon="-0.001"></nd>
		  <nd ref="2" version="3" lat="0.001" lon="0"></nd>
		  <tag k="building" v="yes"></tag>
		  <tag k="name" v="parking garage"></tag>
		  <tag k="height" v="13"></tag>
		 </way>
		</osm>
	`)

	// zoom 13, rounds to 20
	tile := processData(t, data, 13)

	feature := tile["buildings"].Features[0]
	if v := feature.Properties["height"]; v != 20.0 {
		t.Errorf("incorrect height: %v", v)
	}

	// zoom 14, rounds to 10
	tile = processData(t, data, 14)

	feature = tile["buildings"].Features[0]
	if v := feature.Properties["height"]; v != 10.0 {
		t.Errorf("incorrect height: %v", v)
	}

	// zoom 15, rounds to 10
	tile = processData(t, data, 15)

	feature = tile["buildings"].Features[0]
	if v := feature.Properties["height"]; v != 10.0 {
		t.Errorf("incorrect height: %v", v)
	}

	// zoom 16, no rounding
	tile = processData(t, data, 16)

	feature = tile["buildings"].Features[0]
	if v := feature.Properties["height"]; v != 13.0 {
		t.Errorf("incorrect height: %v", v)
	}
}
