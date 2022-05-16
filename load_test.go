package osmzen

import (
	"testing"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osmzen/filter"

	"github.com/paulmach/osm"

	"github.com/pkg/errors"
)

func TestLoadEmbeddedConfig(t *testing.T) {
	config, err := LoadEmbeddedConfig(func(name string) ([]byte, error) {
		return DefaultConfig.ReadFile("config/" + name)
	})
	if err != nil {
		if err, ok := errors.Cause(err).(*filter.CompileError); ok {
			t.Log(err.Error())
			t.Logf("yaml: \n%s", err.YAML())
			t.Logf("%+v", err.Cause)
		}

		t.Fatalf("load error: %v", err)
	}

	testConfig(t, config)
}

func TestLoadDefaultConfig(t *testing.T) {
	config, err := LoadDefaultConfig()
	if err != nil {
		if err, ok := errors.Cause(err).(*filter.CompileError); ok {
			t.Log(err.Error())
			t.Logf("yaml: \n%s", err.YAML())
			t.Logf("%+v", err.Cause)
		}

		t.Fatalf("load error: %v", err)
	}

	testConfig(t, config)
}

func TestLoad(t *testing.T) {
	config, err := Load("config/queries.yaml")
	if err != nil {
		if err, ok := errors.Cause(err).(*filter.CompileError); ok {
			t.Log(err.Error())
			t.Logf("yaml: \n%s", err.YAML())
			t.Logf("%+v", err.Cause)
		}

		t.Fatalf("load error: %v", err)
	}

	testConfig(t, config)
}

func testConfig(t *testing.T, config *Config) {
	o := &osm.OSM{
		Ways: osm.Ways{
			{
				ID: 123,
				Tags: osm.Tags{
					{Key: "building", Value: "yes"},
				},
				Nodes: osm.WayNodes{
					{Lat: 0, Lon: 0},
					{Lat: 1, Lon: 0},
					{Lat: 1, Lon: 1},
					{Lat: 0, Lon: 1},
					{Lat: 0, Lon: 0},
				},
			},
		},
	}

	for i := 10; i < 18; i++ {
		config.Process(o, maptile.New(1, 2, 3).Bound(), 3)
	}
}
