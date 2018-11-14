package filter

import (
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"
)

func TestAnyCond(t *testing.T) {
	filter := parseFilter(t, `
filter:
  - all:
    - building: true
    - not: { building: 'no' }
  - all:
      not: { 'tags->location': 'underground' }`)

	cases := []struct {
		name   string
		tags   map[string]string
		result bool
	}{
		{
			name:   "not match empty tags",
			tags:   map[string]string{},
			result: false,
		},
		{
			name: "not match building no tag",
			tags: map[string]string{
				"building": "no",
			},
			result: false,
		},
		{
			name: "match building yes tag",
			tags: map[string]string{
				"building": "yes",
			},
			result: true,
		},
		{
			name: "match building non-no tag",
			tags: map[string]string{
				"building": "non-no",
			},
			result: true,
		},
		{
			name: "not match with location underground",
			tags: map[string]string{
				"building": "yes",
				"location": "underground",
			},
			result: false,
		},
		{
			name: "match location not underground",
			tags: map[string]string{
				"building": "yes",
				"location": "over ground",
			},
			result: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := filter.Filter.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestBoolCond(t *testing.T) {
	filter := parseFilter(t, `
filter:
  building: false`)

	cases := []struct {
		name   string
		tags   map[string]string
		result bool
	}{
		{
			name:   "no building",
			tags:   map[string]string{},
			result: true,
		},
		{
			name: "yes building",
			tags: map[string]string{
				"building": "foo",
			},
			result: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := filter.Filter.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestStringInCond(t *testing.T) {
	filter := parseFilter(t, `
filter:
  any:
    all:
      not: { operator: 'forest service' }
      protect_class: ['2', '3', '5']`)

	cases := []struct {
		name   string
		tags   map[string]string
		result bool
	}{
		{
			name: "match one of protected classes",
			tags: map[string]string{
				"operator":      "local",
				"protect_class": "2",
			},
			result: true,
		},
		{
			name: "match missing operator with not",
			tags: map[string]string{
				"protect_class": "3",
			},
			result: true,
		},
		{
			name: "match not operator",
			tags: map[string]string{
				"operator":      "forest service",
				"protect_class": "3",
			},
			result: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := filter.Filter.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestWayAreaCond(t *testing.T) {
	filter := parseFilter(t, `
filter:
  way_area: { min: 0.001 }`)

	cases := []struct {
		name   string
		way    *osm.Way
		result bool
	}{
		{
			name: "not match 0 area",
			way: &osm.Way{
				Nodes: osm.WayNodes{
					{Lat: 2, Lon: 2},
					{Lat: 2, Lon: 2},
					{Lat: 2, Lon: 2},
				},
				Tags: osm.Tags{{Key: "building", Value: "yes"}},
			},
			result: false,
		},
		{
			name: "match non-0 area",
			way: &osm.Way{
				Nodes: osm.WayNodes{
					{Lat: 0, Lon: 0},
					{Lat: 0, Lon: 0.1},
					{Lat: 0.1, Lon: 0.1},
					{Lat: 0.1, Lon: 0},
				},
				Tags: osm.Tags{{Key: "building", Value: "yes"}},
			},
			result: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc, err := osmgeojson.Convert(&osm.OSM{Ways: osm.Ways{tc.way}})
			if err != nil {
				t.Errorf("failed to convert to geojson: %v", err)
			}

			ctx := NewContext(nil, fc.Features[0])
			ctx.Debug = true

			v := filter.Filter.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}
