package osmzen

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/paulmach/osmzen/filter"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"

	"github.com/pkg/errors"
)

func TestYAML(t *testing.T) {
	files, err := ioutil.ReadDir("config/yaml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	o := &osm.OSM{
		Nodes: osm.Nodes{
			{
				ID:      1,
				Version: 2,
				Tags: osm.Tags{
					{Key: "amenity", Value: "restaurant"},
					{Key: "cuisine", Value: "burger"},
					{Key: "name", Value: "Kronnerburger"},
				},
			},
		},
		Ways: osm.Ways{
			{
				ID:      1,
				Version: 2,
				Nodes: osm.WayNodes{
					{ID: 1, Lat: 0, Lon: 0},
					{ID: 2, Lat: 0, Lon: 0.001},
					{ID: 3, Lat: 0.001, Lon: 0.001},
					{ID: 4, Lat: 0.001, Lon: 0},
					{ID: 1, Lat: 0, Lon: 0},
				},
				Tags: osm.Tags{
					{Key: "building", Value: "no"},
					{Key: "building:part", Value: "yes"},
				},
			},
		},
	}

	fc, err := osmgeojson.Convert(o)
	if err != nil {
		t.Fatalf("failed to convert to geojson: %v", err)
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}

		filename := "config/yaml/" + f.Name()
		layer, err := loadLayer(filename)
		if err != nil {
			if err, ok := errors.Cause(err).(*filter.CompileError); ok {
				t.Log(err.Error())
				t.Logf("yaml: \n%s", err.YAML())
				t.Logf("%+v", err.Cause)
			}

			t.Errorf("load of %v failed: %v", f.Name(), err)
			continue
		}

		t.Logf("checking %s", filename)
		for _, f := range fc.Features {
			checkLayer(layer, f)
		}
	}
}

func checkLayer(layer *Layer, feature *geojson.Feature) {
	ctx := filter.NewContext(nil, feature)
	ctx.Debug = true

	for _, f := range layer.filters {
		if f.Table != "" && f.Table != "osm" {
			// skip natural earth conditions, only osm
			continue
		}

		f.Match(ctx)

		if f.MinZoom != nil {
			f.MinZoom.EvalNum(ctx)
		}

		// check the outputs
		for _, e := range f.Output {
			e.Expr.Eval(ctx)
		}
	}
}
