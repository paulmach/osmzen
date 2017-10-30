package filter

import (
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/pkg/errors"
)

func TestColExpr(t *testing.T) {
	cases := []struct {
		name   string
		expr   string
		tags   map[string]string
		result interface{}
	}{
		{
			name: "return height tag",
			expr: "col: height",
			tags: map[string]string{
				"height": "8",
			},
			result: 8.0,
		},
		{
			name: "return height from levels",
			expr: "col: height",
			tags: map[string]string{
				"building:levels": "4",
			},
			result: 14.0, // 3*4 + 2
		},
		{
			name:   "return nil height if no tag",
			expr:   "col: height",
			tags:   map[string]string{},
			result: nil,
		},
		{
			name: "returns tag value",
			expr: "col: building",
			tags: map[string]string{
				"building": "for sure",
			},
			result: "for sure",
		},
		{
			name:   "returns nil if tag doesn't exist",
			expr:   "col: building",
			tags:   map[string]string{},
			result: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr := parseExpr(t, tc.expr)
			ctx := NewContext(nil)
			ctx.Tags = tc.tags

			v := expr.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestCaseExpr(t *testing.T) {
	expr := parseExpr(t, `
case:
- when: { not: { 'cycleway': ['no', 'none'] } }
  then: { col: 'cycleway' }
- when: { nono: true }
- when: { 'cycleway': ['no'] }
  then: result
- else: 5`)

	cases := []struct {
		name   string
		tags   map[string]string
		result interface{}
	}{
		{
			name: "return match condition",
			tags: map[string]string{
				"cycleway": "yes",
			},
			result: "yes",
		},
		{
			name: "return nil if not matching no",
			tags: map[string]string{
				"cycleway": "no",
			},
			result: "result",
		},
		{
			name: "return else if not matching none",
			tags: map[string]string{
				"cycleway": "none",
			},
			result: 5.0,
		},
		{
			name: "no expression should be nil",
			tags: map[string]string{
				"cycleway": "no",
				"nono":     "yes",
			},
			result: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(nil)
			ctx.Tags = tc.tags

			v := expr.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestCaseNumExpr(t *testing.T) {
	expr := parseExpr(t, `
case:
- when: { not: { 'cycleway': ['no', 'none'] } }
  then: 1.0
- when: { 'cycleway': ['no'] }
  then: 0
- else: 5`)

	_ = expr.(NumExpression)
}

func TestMinExpr(t *testing.T) {
	expr := parseExpr(t, `
min:
- 10
- { col: 'height' }`)

	cases := []struct {
		name   string
		tags   map[string]string
		result float64
	}{
		{
			name: "return min for first value",
			tags: map[string]string{
				"height": "12",
			},
			result: 10,
		},
		{
			name: "return min when second value",
			tags: map[string]string{
				"height": "8",
			},
			result: 8,
		},
		// TODO: nils in a min??
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(nil)
			ctx.Tags = tc.tags

			v := expr.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestCondExpr(t *testing.T) {
	expr := parseExpr(t, `cond: { height: true }`)

	cases := []struct {
		name   string
		tags   map[string]string
		result interface{}
	}{
		{
			name: "evaluates height present",
			tags: map[string]string{
				"height": "12",
			},
			result: true,
		},
		{
			name:   "evaluates nil if height not present",
			tags:   map[string]string{},
			result: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(nil)
			ctx.Tags = tc.tags

			v := expr.Eval(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func parseExpr(t *testing.T, data string) Expression {
	t.Helper()

	var expr interface{}
	err := yaml.Unmarshal([]byte(data), &expr)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	ce, err := CompileExpression(expr)
	if err != nil {
		switch err := errors.Cause(err).(type) {
		case *CompileError:
			t.Log(err.Error())
			t.Log(err.YAML())
			t.Logf("%+v", err.Cause)
		}

		t.Fatalf("load error: %v", err)
	}

	return ce
}
