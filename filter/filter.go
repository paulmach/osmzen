package filter

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// A Filter is a set of matchers and the resulting tag transformation
// to add to the element properties if the matcher matches.
type Filter struct {
	RawFilter  interface{}            `yaml:"filter"`
	RawOutput  map[string]interface{} `yaml:"output"`
	RawMinZoom interface{}            `yaml:"min_zoom"`
	Table      string                 `yaml:"table"`

	MinZoom NumExpression `yaml:"-"`
	Filter  Condition     `yaml:"-"`
	// Output  map[string]Expression `yaml:"-"`
	Output []OutputExpression `yaml:"-"`
}

// OutputExpression has the key and expression value for the outputs.
// Using a slice vs a key value map was much cleaner.
type OutputExpression struct {
	Key  string
	Expr Expression
}

// Compile will compile the parsed yaml into the expressions
// and conditions. This should be called once before matching.
// Returns a *CompileError with details about what exactly went wrong.
func (f *Filter) Compile() error {
	if f.Table != "" && f.Table != "osm" {
		return nil
	}

	if f.RawMinZoom != nil {
		var err error
		f.MinZoom, err = CompileNumExpression(f.RawMinZoom)
		if err != nil {
			return &CompileError{
				Cause: errors.WithMessage(err, "min_zoom"),
				Input: f.RawMinZoom,
			}
		}
	}

	if f.RawFilter != nil {
		filter, err := CompileCondition(f.RawFilter)
		if err != nil {
			return &CompileError{
				Cause: errors.WithMessage(err, "filter"),
				Input: f.RawFilter,
			}
		}

		f.Filter = filter
	}

	if f.RawOutput != nil {
		f.Output = make([]OutputExpression, 0, len(f.RawOutput))
		for k, v := range f.RawOutput {
			expr, err := CompileExpression(v)
			if err != nil {
				return &CompileError{
					Cause: errors.WithMessage(err, fmt.Sprintf("output %s", k)),
					Input: v,
				}
			}

			if _, ok := expr.(*nilExpr); expr != nil && !ok {
				f.Output = append(f.Output, OutputExpression{
					Key:  k,
					Expr: expr,
				})
			}
		}
	}

	return nil
}

// Match will return true/false if the ctx/element matches the feature.
// Must call Compile() first to initialize the filters.
func (f *Filter) Match(ctx *Context) bool {
	if f.Table != "" && f.Table != "osm" {
		return false
	}

	return f.Filter.Eval(ctx)
}

// Properties of the element mapped to the given filter outputs.
// Must call Compile() first to initialize the output expressions.
func (f *Filter) Properties(ctx *Context) map[string]interface{} {
	result := make(map[string]interface{}, len(f.Output))
	for i := range f.Output {
		o := f.Output[i].Expr.Eval(ctx)
		if o != nil {
			result[f.Output[i].Key] = o
		}
	}

	return result
}

func cleanKey(action string) string {
	return strings.TrimPrefix(action, "tags->")
}
