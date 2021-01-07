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
	if gt := feature.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	expected := map[string]interface{}{
		"min_zoom":    13.0,
		"sort_rank":   475.0,
		"scale_rank":  2.0,
		"height":      23.0,
		"area":        11528.0,
		"volume":      265144.0,
		"kind":        "building",
		"kind_detail": "parking_garage",
		"id":          22942652,
		"type":        "way",
	}

	if !reflect.DeepEqual(feature.Properties, geojson.Properties(expected)) {
		t.Errorf("incorrect properties: %v", feature.Properties)
	}
}
