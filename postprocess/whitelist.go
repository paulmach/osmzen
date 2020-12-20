package postprocess

import (
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osmzen/filter"
	"github.com/pkg/errors"
)

// Applies a whitelist to a particular property on all features in the layer,
// optionally also remapping some values.
type whitelist struct {
	Layer     string
	StartZoom float64
	EndZoom   float64
	Property  string
	Condition filter.Condition
	Whitelist []string
	Remap     map[string]string
}

func (f *whitelist) Eval(ctx *Context, layers map[string]*geojson.FeatureCollection) {
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
			if !f.Condition.Eval(ctx.fctx) {
				continue
			}
		}

		val, ok := feature.Properties[f.Property].(string)
		if !ok {
			continue
		}

		if stringIn(val, f.Whitelist) {
			// leave as is
			continue
		} else if f.Remap != nil {
			if v := f.Remap[val]; v != "" {
				feature.Properties[f.Property] = v
			} else {
				// not in whitelist or remap
				delete(feature.Properties, f.Property)
			}
		} else {
			// not and whitelist and no remap
			delete(feature.Properties, f.Property)
		}
	}
}

func compileWhitelist(ctx *CompileContext, c *Config) (Function, error) {
	f := &whitelist{EndZoom: 50}

	f.Layer = c.Params["layer"].(string)
	zs, ok := c.Params["start_zoom"]
	if ok {
		z, ok := zs.(int)
		if !ok {
			return nil, errors.New("whitelist: start_zoom must be an integer")
		}

		f.StartZoom = float64(z)
	}

	ze, ok := c.Params["end_zoom"]
	if ok {
		z, ok := ze.(int)
		if !ok {
			return nil, errors.New("whitelist: end_zoom must be an integer")
		}

		f.EndZoom = float64(z)
	}

	f.Property, ok = c.Params["property"].(string)
	if !ok {
		return nil, errors.Errorf("whitelist: property required and must be a string: (%T, %v)",
			c.Params["property"], c.Params["property"])
	}

	list, ok := c.Params["whitelist"]
	if !ok {
		return nil, errors.New("whitelist: whitelist is required")
	}
	f.Whitelist = parseStrings(list)

	r, ok1 := c.Params["remap"]
	remap, ok2 := r.(map[interface{}]interface{})
	if ok1 && !ok2 {
		return nil, errors.New("whitelist: remap should be a map")
	}

	if ok1 && ok2 {
		f.Remap = make(map[string]string)
		for k, v := range remap {
			ks, ok := k.(string)
			if !ok {
				return nil, errors.New("whitelist: remap keys must be strings")
			}

			vs, ok := v.(string)
			if !ok {
				return nil, errors.New("whitelist: remap values must be strings")
			}

			f.Remap[ks] = vs
		}
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
