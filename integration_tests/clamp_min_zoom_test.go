package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestClampMinZoom(t *testing.T) {
	tile := processData(t, []byte(`
		<osm>
		 <way id="22942652" visible="true" version="6">
		  <nd ref="1" version="3" lat="0" lon="0"></nd>
		  <nd ref="2" version="3" lat="0.001" lon="0"></nd>
		  <nd ref="3" version="3" lat="0.001" lon="-0.001"></nd>
		  <nd ref="4" version="3" lat="0" lon="-0.001"></nd>
		  <nd ref="1" version="3" lat="0" lon="0"></nd>
		  <tag k="amenity" v="parking"></tag>
		  <tag k="building" v="yes"></tag>
		  <tag k="building:levels" v="7"></tag>
		  <tag k="name" v="parking garage"></tag>
		  <tag k="parking" v="multi-storey"></tag>
		 </way>
		</osm>
	`))

	feature := tile["buildings"].Features[0]
	if gt := feature.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	if v := feature.Properties["scale_rank"]; v != 3.0 {
		// scale rank 3 should map to min_zoom 14
		t.Errorf("incorrect scale_rank: %v", v)
	}

	// from 13 -> 14 because of the camp for scale rank 3
	partialMatch(t, feature.Properties, geojson.Properties{
		"min_zoom": 14.0,
	})
}
