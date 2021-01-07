package postprocess

import (
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osmzen/filter"
	"github.com/pkg/errors"
)

// Maps some values for a particular property to others. Similar to whitelist,
// but won't remove the property if there's no match.
type remap struct {
	Layer     string
	StartZoom float64
	EndZoom   float64
	Property  string
	Condition filter.Condition
	Remap     map[string]string
}

func (f *remap) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
	if ctx.Zoom < f.StartZoom || f.EndZoom < ctx.Zoom {
		return
	}

	layer := layers[f.Layer]
	if layer == nil {
		return
	}

	for _, feature := range layer.Features {
		if f.Condition != nil {
			ctx.fctx = filter.NewContextFromProperties(ctx.fctx, feature.Properties)
			ctx.fctx.Geometry = feature.Geometry
			if !f.Condition.Eval(ctx.fctx) {
				continue
			}
		}

		val, ok := feature.Properties[f.Property].(string)
		if !ok {
			continue
		}

		if v := f.Remap[val]; v != "" {
			feature.Properties[f.Property] = v
		}
	}
}

func compileRemap(ctx *CompileContext, c *Config) (Function, error) {
	f := &remap{EndZoom: 50}

	f.Layer = c.Params["layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("remap: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	ze, ok := c.Params["end_zoom"]
	if ok {
		z, ok := ze.(int)
		if !ok {
			return nil, errors.New("remap: end_zoom must be an integer")
		}

		f.EndZoom = float64(z)
	}

	f.Property, ok = c.Params["property"].(string)
	if !ok {
		return nil, errors.Errorf("remap: property required and must be a string: (%T, %v)",
			c.Params["property"], c.Params["property"])
	}

	r, ok := c.Params["remap"]
	if !ok {
		return nil, errors.New("remap: remap is required")
	}

	remap, ok := r.(map[interface{}]interface{})
	if !ok {
		return nil, errors.New("remap: remap should be a map")
	}

	f.Remap = make(map[string]string)
	for k, v := range remap {
		ks, ok := k.(string)
		if !ok {
			return nil, errors.New("remap: remap keys must be strings")
		}

		vs, ok := v.(string)
		if !ok {
			return nil, errors.New("remap: remap values must be strings")
		}

		f.Remap[ks] = vs
	}

	if c.Params["where"] != nil {
		cond, err := filter.CompileCondition(c.Params["where"])
		if err != nil {
			return nil, err
		}

		f.Condition = cond
	}

	return f, nil
}
