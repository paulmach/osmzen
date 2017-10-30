package filter

import (
	"testing"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"

	yaml "gopkg.in/yaml.v2"
)

func TestFilterProperties(t *testing.T) {
	filter := parseFilter(t, `
output:
  height: {col: height}
  location: {col: tags->location}
  kind: building`)

	o := &osm.OSM{
		Nodes: osm.Nodes{
			{
				ID:  1,
				Lat: 1,
				Lon: 2,
				Tags: osm.Tags{
					{Key: "building", Value: "yes"},
					{Key: "height", Value: "10"},
					{Key: "location", Value: "overground"},
				},
			},
		},
	}

	fc, err := osmgeojson.Convert(o)
	if err != nil {
		t.Errorf("failed to convert to geojson: %v", err)
	}
	ctx := NewContext(fc.Features[0])

	result := filter.Properties(ctx)
	if v := result["kind"]; v != "building" {
		t.Errorf("incorrect kind tag: %v", v)
	}

	if v := result["location"]; v != "overground" {
		t.Errorf("incorrect location tag: %v", v)
	}

	if v := result["height"]; v != 10.0 {
		t.Errorf("incorrect height tag: %v", v)
	}
}

func parseFilter(t *testing.T, data string) *Filter {
	filter := &Filter{}
	err := yaml.Unmarshal([]byte(data), &filter)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	filter.Compile()
	return filter
}
