package postprocess

import (
	"fmt"
	"strings"

	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/geo/geojson"
)

// A Function is the interface implemented by the compiled
// postprocess functions.
type Function interface {
	Eval(*Context, map[string]*geojson.FeatureCollection)
}

// Context represents request level context for the postprocess functions.
type Context struct {
	Zoom  float64 // zoom of the tile request
	Bound geo.Bound
}

// CompileContext is the context to help while compiling.
type CompileContext struct {
	Asset       func(string) ([]byte, error)
	ClipFactors map[string]float64
}

// Config is a set of properties that define the postprocess function.
// This should be compiled into a function that can then be run
// without having to reload/reparse the options.
type Config struct {
	Func      string `yaml:"fn"`
	Resources struct {
		Matcher struct {
			Type     string `yaml:"type"`
			InitFunc string `yaml:"init_fn"`
			Path     string `yaml:"path"`
		} `yaml:"matcher"`
	} `yaml:"resources"`
	Params map[interface{}]interface{} `yaml:"params"`
}

// Compile will transform/parse/build the postprocess function from the config.
func Compile(ctx *CompileContext, c *Config) (Function, error) {
	name := strings.TrimPrefix(c.Func, "vectordatasource.transform.")

	if f, ok := functions[name]; ok {
		if f != nil {
			return f(ctx, c)
		}

		return nil, nil
	}

	panic(fmt.Sprintf("unsupported function: %s", name))
}
