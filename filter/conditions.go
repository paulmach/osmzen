package filter

import (
	"fmt"
	"math"
	"strings"

	"github.com/pkg/errors"
)

// unsupportedConditions are things that we have replaced with
// a different condition and should be ignored by this code.
var unsupportedConditions = map[string]struct{}{
	"way": struct{}{},
}

// A Condition is something that evaluates to a boolean.
type Condition interface {
	Eval(*Context) bool
}

// conditions is the YAML key to the compile function mapping.
var conditionCompilers = map[string]func(interface{}) (Condition, error){}

func init() {
	conditionCompilers = map[string]func(interface{}) (Condition, error){
		"all":            compileAllCond,
		"any":            compileAnyCond,
		"not":            compileNotCond,
		"none":           compileNoneCond,
		"compare":        compileCompareCond,
		"way_area":       compileWayAreaCond,
		"volume":         compileVolumeCond,
		"geometry_types": compileGeometryTypesCond,
		"geom_type":      compileGeometryTypesCond,
		"osm_tags":       compileOSMTagsCond,
	}
}

// CompileCondition will take the parsed YAML condition and
// return a compiled condition interface.
func CompileCondition(cond interface{}) (Condition, error) {
	return compileAllCond(cond)
}

func compilePropertyCond(k interface{}, val interface{}) (Condition, error) {
	key, ok := k.(string)
	if !ok {
		return nil, errors.Errorf("property cond: key is not a string: (%T, %v)", k, k)
	}

	if f, ok := conditionCompilers[key]; ok {
		return f(val)
	}

	// not a rule, so it must be a tag match.
	key = cleanKey(key)
	switch val := val.(type) {
	case bool:
		return &boolCond{Key: key, Val: val}, nil
	case string:
		return &stringCond{Key: key, Val: val}, nil
	case []interface{}:
		c, err := compileStringInCond(key, val)
		return c, errors.WithMessage(err, key)
	default:
		return nil, errors.Errorf("property cond: %s: unsupported type: (%T, %v)", key, val, val)
	}
}

///////////////////////////////////////
// allCond

type allCond []Condition

func (ac allCond) Eval(ctx *Context) bool {
	result := true
	for _, c := range ac {
		if !c.Eval(ctx) {
			result = false
			if !ctx.Debug {
				return false
			}
		}
	}

	return result
}

func compileAllCond(cond interface{}) (Condition, error) {
	var err error
	switch cond := cond.(type) {
	case []interface{}:
		ac := make(allCond, len(cond))
		for i, c := range cond {
			ac[i], err = CompileCondition(c)
			if err != nil {
				return nil, errors.WithMessage(err, "all")
			}
		}

		return ac, nil
	case map[interface{}]interface{}:
		ac := make(allCond, 0, len(cond))

		for k, v := range cond {
			key, ok := k.(string)
			if !ok {
				return nil, errors.Errorf("keys must be strings: (%T, %v)", k, k)
			}

			if _, ok := unsupportedConditions[key]; ok {
				continue
			}

			cc, err := compilePropertyCond(k, v)
			if err != nil {
				return nil, errors.WithMessage(err, "all")
			}

			ac = append(ac, cc)
		}

		return ac, nil
	default:
		return nil, errors.Errorf("all: unsupported type: (%T, %v)", cond, cond)
	}
}

///////////////////////////////////////
// anyCond

type anyCond []Condition

func (ac anyCond) Eval(ctx *Context) bool {
	result := false
	for _, c := range ac {
		if c.Eval(ctx) {
			result = true
			if !ctx.Debug {
				return true
			}
		}
	}

	return result
}

func compileAnyCond(cond interface{}) (Condition, error) {
	var err error
	switch cond := cond.(type) {
	case []interface{}:
		ac := make(anyCond, len(cond))
		for i, c := range cond {
			ac[i], err = CompileCondition(c)
			if err != nil {
				return nil, errors.WithMessage(err, "any")
			}
		}

		return ac, nil
	case map[interface{}]interface{}:
		ac := make(anyCond, len(cond))

		i := 0
		for k, v := range cond {
			cc, err := compilePropertyCond(k, v)
			if err != nil {
				return nil, errors.WithMessage(err, "any")
			}

			ac[i] = cc
			i++
		}

		return ac, nil
	default:
		return nil, errors.Errorf("any: unsupported type: (%T, %v)", cond, cond)
	}
}

///////////////////////////////////////
// notCond

type notCond struct {
	Condition Condition
}

func (nc *notCond) Eval(ctx *Context) bool {
	return !nc.Condition.Eval(ctx)
}

func compileNotCond(cond interface{}) (Condition, error) {
	c, err := CompileCondition(cond)
	if err != nil {
		return nil, errors.WithMessage(err, "not")
	}
	return &notCond{Condition: c}, nil
}

///////////////////////////////////////
// noneCond

func compileNoneCond(cond interface{}) (Condition, error) {
	c, err := compileAnyCond(cond)
	if err != nil {
		return nil, errors.WithMessage(err, "none")
	}
	return &notCond{Condition: c}, nil
}

///////////////////////////////////////
// osmTagsCond

type osmTagsCond struct {
	Condition Condition
}

func (oc *osmTagsCond) Eval(ctx *Context) bool {
	tags := ctx.Tags

	// Use the osm tags from now on
	ctx.Tags = ctx.OSMTags

	val := oc.Condition.Eval(ctx)
	ctx.Tags = tags

	return val
}

func compileOSMTagsCond(cond interface{}) (Condition, error) {
	c, err := CompileCondition(cond)
	if err != nil {
		return nil, errors.WithMessage(err, "osm_tags")
	}
	return &osmTagsCond{Condition: c}, nil
}

///////////////////////////////////////
// geometryTypesCond

type geometryTypesCond struct {
	Types []string
}

func (gtc *geometryTypesCond) Eval(ctx *Context) bool {
	val := strings.ToLower(ctx.Geometry.GeoJSONType())
	for _, t := range gtc.Types {
		if t == val {
			return true
		}
	}

	return false
}

func compileGeometryTypesCond(cond interface{}) (Condition, error) {
	var condArray []interface{}
	switch c := cond.(type) {
	case []interface{}:
		condArray = c
	case string:
		condArray = []interface{}{c}
	default:
		return nil, errors.Errorf("geometry_types/geom_type: requires array of strings or string: (%T, %v)", cond, cond)
	}

	types := make([]string, len(condArray))
	for i, c := range condArray {
		if s, ok := c.(string); ok {
			types[i] = strings.ToLower(s)
		} else {
			return nil, errors.Errorf("geometry_types: requires array of strings: (%T, %v)", c, c)
		}
	}
	return &geometryTypesCond{Types: types}, nil
}

///////////////////////////////////////
// wayAreaCond

type wayAreaCond struct {
	MinMax *minMaxCond
}

func (wac *wayAreaCond) Eval(ctx *Context) bool {
	return wac.MinMax.Eval(ctx.Area())
}

func compileWayAreaCond(cond interface{}) (Condition, error) {
	cc, err := compileMinMaxCond(cond)
	if err != nil {
		return nil, errors.WithMessage(err, "way_area")
	}

	return &wayAreaCond{MinMax: cc}, nil
}

///////////////////////////////////////
// volumeCond

type volumeCond struct {
	MinMax *minMaxCond
}

func (vc *volumeCond) Eval(ctx *Context) bool {
	return vc.MinMax.Eval(ctx.Height() * ctx.Area())
}

func compileVolumeCond(cond interface{}) (Condition, error) {
	cc, err := compileMinMaxCond(cond)
	if err != nil {
		return nil, errors.WithMessage(err, "volume")
	}

	return &volumeCond{MinMax: cc}, nil
}

///////////////////////////////////////
// minMaxCond

type minMaxCond struct {
	Min, Max float64
}

func (mmc *minMaxCond) Eval(val float64) bool {
	if val > mmc.Max {
		return false
	}

	if val < mmc.Min {
		return false
	}

	return true
}

func compileMinMaxCond(cond interface{}) (*minMaxCond, error) {
	hash, ok := cond.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("minmax: hash required (eg. { min: , max: }): (%T, %v)", cond, cond)
	}

	mmc := &minMaxCond{
		Min: -math.MaxFloat64,
		Max: math.MaxFloat64,
	}

	var err error
	if v, ok := hash["min"]; ok {
		mmc.Min, err = parseFloat64(v)
		if err != nil {
			return nil, errors.Errorf("minmax: min is not a number: (%T, %v)", v, v)
		}
	}

	if v, ok := hash["max"]; ok {
		mmc.Max, err = parseFloat64(v)
		if err != nil {
			return nil, errors.Errorf("minmax: max is not a number: (%T, %v)", v, v)
		}
	}

	return mmc, nil
}

///////////////////////////////////////
// compareCond

type compareCond struct {
	Left, Right NumExpression
	Operator    string
}

func (cc *compareCond) Eval(ctx *Context) bool {
	left := cc.Left.EvalNum(ctx)
	right := cc.Right.EvalNum(ctx)
	switch cc.Operator {
	case "lt":
		return left < right
	case "gt":
		return left > right
	case "lte":
		return left <= right
	case "gte":
		return left >= right
	}

	// impossible, wrong value will be caught during compile.
	panic(fmt.Sprintf("unsupported operator: %v", cc.Operator))
}

func compileCompareCond(cond interface{}) (Condition, error) {
	cc := &compareCond{}
	parts, ok := cond.([]interface{})
	if !ok || len(parts) != 3 {
		return nil, errors.Errorf("compare: requires 3 part array (eg. [3,'lt',5]): (%T, %v)", cond, cond)
	}

	var err error
	cc.Left, err = CompileNumExpression(parts[0])
	if err != nil {
		return nil, errors.WithMessage(err, "compare")
	}

	cc.Right, err = CompileNumExpression(parts[2])
	if err != nil {
		return nil, errors.WithMessage(err, "compare")
	}

	cc.Operator, ok = parts[1].(string)
	if !ok || (cc.Operator != "lt" && cc.Operator != "gt" &&
		cc.Operator != "lte" && cc.Operator != "gte") {

		return nil, errors.Errorf("compare: invalid operate (ie, not 'lt', 'gt', 'lte' or 'gte'): (%T, %v)", parts[1], parts[1])
	}

	return cc, nil
}

///////////////////////////////////////
// stringCond

// eg. building: 'no' (building tag == 'no')
type stringCond struct {
	Key string
	Val string
}

func (sc *stringCond) Eval(ctx *Context) bool {
	return ctx.Tags[sc.Key] == sc.Val
}

///////////////////////////////////////
// stringInCond

// eg. protect_class: [2, 3, 5] (one of these values)
type stringInCond struct {
	Key  string
	List []string
}

func (sc *stringInCond) Eval(ctx *Context) bool {
	val := ctx.Tags[sc.Key]
	for _, l := range sc.List {
		if val == l {
			return true
		}
	}

	return false
}

func compileStringInCond(key string, list []interface{}) (*stringInCond, error) {
	sl := make([]string, len(list))
	for i, l := range list {
		if s, ok := l.(string); ok {
			sl[i] = strings.ToLower(s)
		} else {
			return nil, errors.Errorf("string in: requires array of strings: (%T, %v)", list, list)
		}
	}

	return &stringInCond{Key: key, List: sl}, nil
}

///////////////////////////////////////
// boolCond

// eg. building: true (building tag exists)
type boolCond struct {
	Key string
	Val bool
}

func (bc *boolCond) Eval(ctx *Context) bool {
	_, ok := ctx.Tags[bc.Key]
	return ok == bc.Val
}
