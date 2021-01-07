package integrationtests

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestSetConditionalNames(t *testing.T) {
	data := []byte(`
		<osm>
		 <way id="123" visible="true" version="6">
		  <nd ref="2" version="3" lat="0.001" lon="0"></nd>
		  <nd ref="3" version="3" lat="0.001" lon="-0.001"></nd>
		  <nd ref="4" version="3" lat="0" lon="-0.001"></nd>
		  <nd ref="2" version="3" lat="0.001" lon="0"></nd>
		  <tag k="area" v="yes"></tag>
		  <tag k="zoo" v="petting_zoo"></tag>
		  <tag k="surface" v="dirt"></tag>
		  <tag k="name" v="osm zoo"></tag>
		  <tag k="name:en" v="osm zoo"></tag>
		  <tag k="old_name:en" v="osm zoo"></tag>
		  <tag k="short_name" v="osmz"></tag>
		  <tag k="name:short" v="osmz"></tag>
		 </way>
		</osm>
	`)

	// zoo becomes a poi and a landuse. poi gets the label so
	// the landuse should not have a name
	tile := processData(t, data, 17)

	landuse := tile["landuse"].Features[0]
	if gt := landuse.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	// landuse should not have name
	if v, ok := landuse.Properties["name"]; ok {
		t.Errorf("name should not be present: %v", v)
	}

	poi := tile["pois"].Features[0]
	if gt := landuse.Geometry.GeoJSONType(); gt != geojson.TypePolygon {
		t.Errorf("incorrect geometry type: %v != %v", gt, geojson.TypePolygon)
	}

	// landuse should not have name
	if _, ok := poi.Properties["name"]; !ok {
		t.Errorf("does not have name")
	}
}
