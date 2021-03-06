package integrationtests

import (
	"encoding/xml"
	"reflect"
	"testing"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
	"github.com/paulmach/osmzen"
)

var config *osmzen.Config

func init() {
	var err error
	config, err = osmzen.Load("../config/queries.yaml")
	if err != nil {
		panic(err)
	}
}

func partialMatch(t *testing.T, actual, expected geojson.Properties) {
	t.Helper()

	var hasError bool
	for k, ev := range expected {
		av, ok := actual[k]
		if !ok {
			t.Errorf("'%s' is not in actual", k)
			hasError = true
		} else if !reflect.DeepEqual(av, ev) {
			t.Errorf("'%s' is not equal", k)
			hasError = true
		}
	}

	if hasError {
		t.Logf("actual:   %v", actual)
		t.Logf("expected: %v", expected)
	}
}

func processData(t *testing.T, data []byte, zooms ...int) map[string]*geojson.FeatureCollection {
	o := &osm.OSM{}
	err := xml.Unmarshal([]byte(data), &o)
	if err != nil {
		t.Fatalf("unable to unmarshal xml: %v", err)
	}

	return processOSM(t, o, zooms...)
}

func processOSM(t *testing.T, o *osm.OSM, zooms ...int) map[string]*geojson.FeatureCollection {
	zoom := 20
	if len(zooms) != 0 {
		zoom = zooms[0]
	}

	tile, err := config.Process(
		o, maptile.Tile{}.Bound(), maptile.Zoom(zoom),
	)
	if err != nil {
		t.Fatalf("unable to build geojson: %v", err)
	}

	return tile
}
