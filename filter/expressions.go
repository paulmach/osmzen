package filter

import (
	"fmt"
	"math"
	"strings"

	"github.com/pkg/errors"
)

// unsupportedExpressions are things that we have replaced with
// a different expression and should be ignored by this code.
var unsupportedExpressions = map[string]struct{}{
	"expr": struct{}{},
}

// An Expression is something that evaluates to a boolean,
// number or string, depending on context.
type Expression interface {
	Eval(*Context) interface{}
}

// NumExpression is something to evaluates to a number.
type NumExpression interface {
	EvalNum(*Context) float64
}

// expression is the YAML key to the compile function mapping.
var expressions = map[string]func(interface{}) (Expression, error){}

func init() {
	expressions = map[string]func(interface{}) (Expression, error){
		"col":    compileColExpr,
		"call":   compileCallExpr,
		"case":   compileCaseExpr,
		"clamp":  compileClampExpr,
		"lookup": compileLookupExpr,
		"min":    compileMinExpr,
		"max":    compileMaxExpr,
		"sum":    compileSumExpr,
		"mul":    compileMulExpr,
		"cond":   compileCondExpr,
	}
}

// CompileNumExpression will compile the parsed YAML into
// a NumExpression that can be evaluated.
func CompileNumExpression(expr interface{}) (NumExpression, error) {
	expr, err := CompileExpression(expr)
	if err != nil {
		return nil, err
	}

	if ne, ok := expr.(NumExpression); ok {
		return ne, nil
	}

	return nil, errors.Errorf("not numeric: (%T, %v)", expr, expr)
}

// CompileExpression will compile the parsed YAML into
// an Expression that can be evaluated into a bool, float64 or string.
func CompileExpression(expr interface{}) (Expression, error) {
	if expr == nil {
		return nilExpr{}, nil
	}

	switch expr := expr.(type) {
	case int:
		return numExpr{Val: float64(expr)}, nil
	case float64:
		return numExpr{Val: expr}, nil
	case bool:
		return boolExpr{Val: expr}, nil
	case string:
		if expr != "" {
			return stringExpr{Val: expr}, nil
		}

		return nilExpr{}, nil
	case map[interface{}]interface{}:
		// The col expression is the most typical for output
		// and it just looks like { col: 'tags->location' }
		if len(expr) != 1 {
			for k := range unsupportedExpressions {
				delete(expr, k)
			}
		}

		if len(expr) == 1 {
			for k, v := range expr {
				key, ok := k.(string)
				if !ok {
					return nil, errors.Errorf("key must be a string: (%T, %v)", k, k)
				}

				if f, ok := expressions[key]; ok {
					ce, err := f(v)
					return ce, err
				}

				return nil, errors.Errorf("unsupported type: %s", key)
			}
		}

		return nil, errors.Errorf("multiple properties: %v", expr)
	default:
		// some sort of unsupported yaml structure
		// in the outputs section.
		return nil, errors.Errorf("unsupported type: (%T, %v)", expr, expr)
	}
}

///////////////////////////////////////
// heightExpr

type heightExpr struct{}

func (he *heightExpr) Eval(ctx *Context) interface{} {
	h := he.EvalNum(ctx)
	if h == 0 {
		return nil
	}

	return h
}

func (he *heightExpr) EvalNum(ctx *Context) float64 {
	return ctx.Height()
}

///////////////////////////////////////
// colExpr

type colExpr struct {
	Key string
}

func (ce *colExpr) Eval(ctx *Context) interface{} {
	if val := ctx.Tags[ce.Key]; val != "" {
		return val
	}

	return nil
}

var colExpressions = map[string]Expression{
	"height":             &heightExpr{},
	"zoom":               &zoomExpr{},
	"area":               &areaExpr{},
	"way_area":           &areaExpr{},
	"is_bus_route":       &calculateIsBusRoute{},
	"mz_cycling_network": &cyclingNetwork{},
	"mz_networks":        &getRelNetworks{},
	"mz_is_building":     &calculateIsBuildingOrPart{},

	// TODO: transit stuf I don't understand
	"mz_transit_score":            &nilExpr{},
	"mz_transit_root_relation_id": &nilExpr{},
}

func compileColExpr(expr interface{}) (Expression, error) {
	// Represents a tag mapped to a postgres column,
	// for us it's just a tag.
	// Example: `roof_color: {col: "roof:color"}`
	e, ok := expr.(string)
	if !ok {
		return nil, errors.Errorf("col: value must be string: (%T, %v)", expr, expr)
	}

	key := cleanKey(e)
	if e, ok := colExpressions[key]; ok {
		return e, nil
	}

	unsupported := []string{"mz_label_placement", "mz_n_photos"}
	if strings.HasPrefix(key, "mz_") && !stringIn(key, unsupported) {
		// vector-datasource will cache function result as column values.
		// we just run the function, but we panic if it's new and don't know about it.
		panic(fmt.Sprintf("unsupported mapzen function/column: %s", key))
	}

	return &colExpr{Key: key}, nil
}

///////////////////////////////////////
// callExpr

func compileCallExpr(expr interface{}) (Expression, error) {
	exprMap, ok := expr.(map[interface{}]interface{})
	if !ok || len(exprMap) != 2 {
		return nil, errors.Errorf("call: must be a hash (eg. { func: , args: [] }): (%T, %v)", expr, expr)
	}

	name, ok := exprMap["func"].(string)
	if !ok {
		return nil, errors.Errorf("call: function name not a string: (%T, %v)", exprMap["func"], exprMap["func"])
	}

	f, ok := functions[name]
	if !ok {
		return nil, errors.Errorf("call: function not defined: %v", name)
	}

	exprs, ok := exprMap["args"].([]interface{})
	if !ok {
		return nil, errors.Errorf("call: args are not an array: (%T, %v)", exprMap["args"], exprMap["args"])
	}

	args := make([]Expression, len(exprs))
	for i, e := range exprs {
		ce, err := CompileExpression(e)
		if err != nil {
			return nil, errors.WithMessage(err, "call")
		}

		args[i] = ce
	}

	r, err := f(args)
	return r, errors.WithMessage(err, fmt.Sprintf("func: %s", name))
}

///////////////////////////////////////
// caseExpr

type caseExpr struct {
	Whens []Condition
	Thens []Expression
	Else  Expression
}

func (ce *caseExpr) Eval(ctx *Context) interface{} {
	var result interface{}

	found := false
	for i, w := range ce.Whens {
		if w.Eval(ctx) {
			result = ce.Thens[i].Eval(ctx)
			found = true
			if !ctx.Debug {
				return result
			}
		}
	}

	if ce.Else != nil {
		if !found {
			return ce.Else.Eval(ctx)
		} else if ctx.Debug {
			ce.Else.Eval(ctx)
		}
	}

	if found {
		return result
	}

	return nil
}

///////////////////////////////////////
// numCaseExpr

type numCaseExpr struct {
	Whens []Condition
	Thens []NumExpression
	Else  NumExpression
}

func (nce *numCaseExpr) Eval(ctx *Context) interface{} {
	return nce.EvalNum(ctx)
}

func (nce *numCaseExpr) EvalNum(ctx *Context) float64 {
	result := 0.0
	found := false

	for i, w := range nce.Whens {
		if w.Eval(ctx) {
			result = nce.Thens[i].EvalNum(ctx)
			found = true
			if !ctx.Debug {
				return result
			}
		}
	}

	if nce.Else != nil {
		if !found {
			return nce.Else.EvalNum(ctx)
		} else if ctx.Debug {
			nce.Else.EvalNum(ctx)
		}
	}

	if found {
		return result
	}

	panic(fmt.Sprintf("case: did not match any conditions: %v", nce))
}

func compileCaseExpr(expr interface{}) (Expression, error) {
	cases, ok := expr.([]interface{})
	if !ok {
		return nil, errors.Errorf("case: must be array of { when: , then: }: (%T, %v)", expr, expr)
	}

	ce := &caseExpr{
		Whens: make([]Condition, 0, len(cases)),
		Thens: make([]Expression, 0, len(cases)),
	}

	isNum := true
	for _, c := range cases {
		cas, ok := c.(map[interface{}]interface{})
		if !ok {
			return nil, errors.Errorf("case: condition must be of the form { when: , then: }: (%T, %v)", c, c)
		}

		when := cas["when"]
		then := cas["then"]
		els := cas["else"]

		if els != nil {
			if when != nil || then != nil {
				return nil, errors.Errorf("case: condition must be of the form { when: , then: }: (%T, %v)", c, c)
			}

			var err error
			ce.Else, err = CompileExpression(els)
			if err != nil {
				return nil, errors.WithMessage(err, "case: else")
			}

			if _, ok := ce.Else.(NumExpression); !ok {
				isNum = false
			}
			continue
		}

		if when == nil {
			return nil, errors.Errorf("case: condition must be of the form { when: , then: }: (%T, %v)", c, c)
		}

		cw, err := CompileCondition(when)
		if err != nil {
			return nil, errors.WithMessage(err, "case: when")
		}
		ce.Whens = append(ce.Whens, cw)

		if then == nil {
			ce.Thens = append(ce.Thens, nilExpr{})
			isNum = false
		} else {
			ct, err := CompileExpression(then)
			if err != nil {
				return nil, errors.WithMessage(err, "case: then")
			}

			ce.Thens = append(ce.Thens, ct)
			if _, ok := ct.(NumExpression); !ok {
				isNum = false
			}
		}
	}

	// We try to upgrade this case condition to a NumExpression
	// if all the values are numbers. Should remove one level of casting.
	if isNum {
		nes := make([]NumExpression, len(ce.Thens))
		for i, e := range ce.Thens {
			nes[i] = e.(NumExpression)
		}

		ncs := &numCaseExpr{
			Whens: ce.Whens,
			Thens: nes,
		}

		if ce.Else != nil {
			ncs.Else = ce.Else.(NumExpression)
		}

		return ncs, nil
	}

	return ce, nil
}

///////////////////////////////////////
// clampExpr

type clampExpr struct {
	Min   NumExpression
	Max   NumExpression
	Value NumExpression
}

func (expr *clampExpr) Eval(ctx *Context) interface{} {
	return expr.EvalNum(ctx)
}

func (expr *clampExpr) EvalNum(ctx *Context) float64 {
	val := expr.Value.EvalNum(ctx)

	if min := expr.Min.EvalNum(ctx); val < min {
		return min
	}

	if max := expr.Max.EvalNum(ctx); val > max {
		return max
	}

	return val
}

func compileClampExpr(expr interface{}) (Expression, error) {
	options, ok := expr.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("clamp: must be hash of the form { min: , max: , value: }: (%T, %v)", expr, expr)
	}

	minExpr := options["min"]
	maxExpr := options["max"]
	valueExpr := options["value"]

	if minExpr == nil || maxExpr == nil || valueExpr == nil {
		return nil, errors.Errorf("clamp: must be hash of the form { min: , max: , value: }: (%T, %v)", expr, expr)
	}

	min, err := CompileNumExpression(minExpr)
	if err != nil {
		return nil, errors.WithMessage(err, "clamp: min")
	}

	max, err := CompileNumExpression(maxExpr)
	if err != nil {
		return nil, errors.WithMessage(err, "clamp: max")
	}

	value, err := CompileNumExpression(valueExpr)
	if err != nil {
		return nil, errors.WithMessage(err, "clamp: value")
	}

	return &clampExpr{
		Min:   min,
		Max:   max,
		Value: value,
	}, nil
}

///////////////////////////////////////
// minExpr

type minExpr struct {
	Expressions []NumExpression
}

func (expr *minExpr) Eval(ctx *Context) interface{} {
	return expr.EvalNum(ctx)
}

func (expr *minExpr) EvalNum(ctx *Context) float64 {
	min := math.MaxFloat64

	for _, e := range expr.Expressions {
		if v := e.EvalNum(ctx); v < min {
			min = v
		}
	}

	return min
}

func compileMinExpr(expr interface{}) (Expression, error) {
	ce, err := compileNumExpressionList(expr)
	if err != nil {
		return nil, errors.WithMessage(err, "min")
	}

	return &minExpr{Expressions: ce}, nil
}

///////////////////////////////////////
// maxExpr

type maxExpr struct {
	Expressions []NumExpression
}

func (expr *maxExpr) Eval(ctx *Context) interface{} {
	return expr.EvalNum(ctx)
}

func (expr *maxExpr) EvalNum(ctx *Context) float64 {
	max := -math.MaxFloat64

	for _, e := range expr.Expressions {
		if v := e.EvalNum(ctx); v > max {
			max = v
		}
	}

	return max
}

func compileMaxExpr(expr interface{}) (Expression, error) {
	ce, err := compileNumExpressionList(expr)
	if err != nil {
		return nil, errors.WithMessage(err, "max")
	}

	return &maxExpr{Expressions: ce}, nil
}

///////////////////////////////////////
// condExpr

type condExpr struct {
	Condition Condition
}

func (ce *condExpr) Eval(ctx *Context) interface{} {
	v := ce.Condition.Eval(ctx)
	if !v {
		return nil
	}

	return true
}

func compileCondExpr(expr interface{}) (Expression, error) {
	c, err := CompileCondition(expr)
	if err != nil {
		return nil, errors.WithMessage(err, "cond")
	}
	return &condExpr{Condition: c}, nil
}

///////////////////////////////////////
// zoomExpr

type zoomExpr struct{}

func (z zoomExpr) Eval(ctx *Context) interface{} {
	return ctx.MinZoom()
}

func (z zoomExpr) EvalNum(ctx *Context) float64 {
	return ctx.MinZoom()
}

///////////////////////////////////////
// areaExpr

type areaExpr struct{}

func (a areaExpr) Eval(ctx *Context) interface{} {
	return ctx.Area()
}

func (a areaExpr) EvalNum(ctx *Context) float64 {
	return ctx.Area()
}

///////////////////////////////////////
// sumExpr

type sumExpr struct {
	Expressions []NumExpression
}

func (se *sumExpr) Eval(ctx *Context) interface{} {
	return se.EvalNum(ctx)
}

func (se *sumExpr) EvalNum(ctx *Context) float64 {
	result := 0.0

	for _, e := range se.Expressions {
		result += e.EvalNum(ctx)
	}

	return result
}

func compileSumExpr(expr interface{}) (Expression, error) {
	ce, err := compileNumExpressionList(expr)
	if err != nil {
		return nil, errors.WithMessage(err, "sum")
	}
	return &sumExpr{Expressions: ce}, nil
}

///////////////////////////////////////
// mulExpr

type mulExpr struct {
	Expressions []NumExpression
}

func (me *mulExpr) Eval(ctx *Context) interface{} {
	return me.EvalNum(ctx)
}

func (me *mulExpr) EvalNum(ctx *Context) float64 {
	result := 1.0

	for _, e := range me.Expressions {
		result *= e.EvalNum(ctx)
	}

	return result
}

func compileMulExpr(expr interface{}) (Expression, error) {
	ce, err := compileNumExpressionList(expr)
	if err != nil {
		return nil, errors.WithMessage(err, "mul")
	}

	return &mulExpr{Expressions: ce}, nil
}

func compileNumExpressionList(expr interface{}) ([]NumExpression, error) {
	exprs, ok := expr.([]interface{})
	if !ok || len(exprs) == 0 {
		return nil, errors.Errorf("must be array of numbers: (%T, %v)", expr, expr)
	}

	result := make([]NumExpression, len(exprs))
	for i, e := range exprs {
		ce, err := CompileNumExpression(e)
		if err != nil {
			return nil, errors.WithMessage(err, "expr")
		}

		result[i] = ce
	}

	return result, nil
}

///////////////////////////////////////
// numExpr

type numExpr struct {
	Val float64
}

func (ne numExpr) Eval(ctx *Context) interface{} {
	return ne.EvalNum(ctx)
}

func (ne numExpr) EvalNum(ctx *Context) float64 {
	return ne.Val
}

///////////////////////////////////////
// stringExpr

type stringExpr struct {
	Val string
}

func (se stringExpr) Eval(ctx *Context) interface{} {
	return se.Val
}

///////////////////////////////////////
// boolExpr

type boolExpr struct {
	Val bool
}

func (be boolExpr) Eval(ctx *Context) interface{} {
	return be.Val
}

///////////////////////////////////////
// nilExpr

type nilExpr struct{}

func (ne nilExpr) Eval(ctx *Context) interface{} {
	return nil
}
