package postprocess

import (
	"github.com/paulmach/orb/geojson"
	"github.com/pkg/errors"
)

// Clamps the min zoom for features depending on context.
// Pushes items into higher/more detailed zoom levels if their
// scale_rank is bad (high).
type clampMinZoom struct {
	Layer     string
	StartZoom float64
	EndZoom   float64
	Property  string
	Clamp     map[float64]float64
}

func (f *clampMinZoom) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom || f.EndZoom < ctx.Zoom {
		return
	}

	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	for _, feature := range layer.Features {
		val, ok := feature.Properties[f.Property].(float64)
		if !ok {
			continue
		}
		minZoom, ok := feature.Properties["min_zoom"].(float64)
		if !ok {
			continue
		}

		clampedZoom, ok := f.Clamp[val]
		if !ok {
			continue
		}

		if minZoom < clampedZoom {
			feature.Properties["min_zoom"] = clampedZoom
		}
	}
}

func compileClampMinZoom(ctx *CompileContext, c *Config) (Function, error) {
	f := &clampMinZoom{EndZoom: 50, Clamp: make(map[float64]float64)}

	f.Layer = c.Params["layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("clamp_min_zoom: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	ze, ok := c.Params["end_zoom"]
	if ok {
		z, ok := ze.(int)
		if !ok {
			return nil, errors.New("clamp_min_zoom: end_zoom must be an integer")
		}

		f.EndZoom = float64(z)
	}

	f.Property, ok = c.Params["property"].(string)
	if !ok {
		return nil, errors.Errorf("clamp_min_zoom: property required and must be a string: (%T, %v)",
			c.Params["property"], c.Params["property"])
	}

	clamp, ok := c.Params["clamp"].(map[interface{}]interface{})
	if !ok {
		return nil, errors.New("clamp_min_zoom: is required")
	}

	for k, v := range clamp {
		ki, ok1 := k.(int)
		vi, ok2 := v.(int)
		if ok1 && ok2 {
			f.Clamp[float64(ki)] = float64(vi)
		} else {
			return nil, errors.New("clamp_min_zoom: clamp keys and values must be integers")
		}
	}

	return f, nil
}
