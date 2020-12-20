package postprocess

import (
	"math"
	"reflect"

	"github.com/paulmach/orb/geojson"
	"github.com/pkg/errors"
)

type quantizeHeight struct {
	Layer     string
	StartZoom float64
	EndZoom   float64
}

func quantize(val, step float64) float64 {
	// special case: if val is very small, we don't want it rounding to zero, so
	// round the smallest values up to the first step.
	if val < step {
		return math.Floor(step)
	}

	return math.Floor(step * math.Round(val/step))
}

func (f *quantizeHeight) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom || f.EndZoom < ctx.Zoom {
		return
	}

	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	if ctx.Zoom == 13 {
		for _, feature := range layer.Features {
			if height, ok := feature.Properties["height"].(float64); ok {
				feature.Properties["height"] = quantize(height, 20)
			}
		}
	}

	if ctx.Zoom == 14 {
		for _, feature := range layer.Features {
			if height, ok := feature.Properties["height"].(float64); ok {
				feature.Properties["height"] = quantize(height, 10)
			}
		}
	}

	if ctx.Zoom == 15 {
		for _, feature := range layer.Features {
			if height, ok := feature.Properties["height"].(float64); ok {
				feature.Properties["height"] = quantize(height, 10)
			}
		}
	}
}

func compileQuantizeHeight(ctx *CompileContext, c *Config) (Function, error) {
	f := &quantizeHeight{EndZoom: 50}

	f.Layer = c.Params["source_layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("quantize_height: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	ze, ok := c.Params["end_zoom"]
	if ok {
		z, ok := ze.(int)
		if !ok {
			return nil, errors.New("quantize_height: end_zoom must be an integer")
		}

		f.EndZoom = float64(z)
	}

	quantize, ok := c.Params["quantize"].(map[interface{}]interface{})
	if !ok {
		return nil, errors.New("quantize_height: is required")
	}

	// behavior is hard coded so will need to do something different if things change.
	expected := map[interface{}]interface{}{
		13: "vectordatasource.transform.quantize_height_round_nearest_20_meters",
		14: "vectordatasource.transform.quantize_height_round_nearest_10_meters",
		15: "vectordatasource.transform.quantize_height_round_nearest_10_meters",
	}
	if !reflect.DeepEqual(quantize, expected) {
		return nil, errors.New("quantize_height: quanize has changed")
	}

	return f, nil
}
