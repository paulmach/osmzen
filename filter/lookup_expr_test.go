package filter

import "testing"

func TestLookup(t *testing.T) {
	expr := parseExpr(t, `
lookup:
  key: { col: height }
  op: '>='
  table:
    - [ 1, 10000 ]
    - [ 2, 1000 ]
    - [ 'abc', 10 ]
    - [ 4, 1 ]
  default: { col: location }`)

	cases := []struct {
		name   string
		height string
		result interface{}
	}{
		{
			name:   "match regular value",
			height: "1000",
			result: 2,
		},
		{
			name:   "match non number value",
			height: "500",
			result: "abc",
		},
		{
			name:   "default",
			height: "0",
			result: "overground",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(nil)
			ctx.Tags = map[string]string{
				"height":   tc.height,
				"location": "overground",
			}

			v := expr.Eval(ctx)
			if v != tc.result {
				t.Logf("%T", v)
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestLookupNumExprLTE(t *testing.T) {
	expr := parseExpr(t, `
lookup:
  key: { col: height }
  op: '<='
  table:
    - [ 1, 10 ]
    - [ 2, 100 ]
    - [ 3, 1000 ]
  default: 4`)

	cases := []struct {
		name   string
		height string
		result float64
	}{
		{
			name:   "match regular value",
			height: "100",
			result: 2,
		},
		{
			name:   "first match",
			height: "1",
			result: 1,
		},
		{
			name:   "default",
			height: "10000",
			result: 4,
		},
	}

	if _, ok := expr.(*lookupNumExprLTE); !ok {
		t.Fatalf("should be num lte expr: %T", expr)
	}

	ne := expr.(NumExpression) // should convert to NumExpression
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(nil)
			ctx.Tags = map[string]string{
				"height": tc.height,
			}

			i := expr.Eval(ctx)
			if i != tc.result {
				t.Errorf("wrong result: %v != %v", i, tc.result)
			}

			v := ne.EvalNum(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}
