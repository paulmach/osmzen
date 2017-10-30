package filter

import "github.com/pkg/errors"

type lookupExpr struct {
	Key     NumExpression
	Op      string
	Values  []float64
	Thens   []interface{}
	Default Expression
}

func (expr *lookupExpr) Eval(ctx *Context) interface{} {
	key := expr.Key.EvalNum(ctx)

	for i, val := range expr.Values {
		matched := false
		switch expr.Op {
		case ">=":
			if key >= val {
				matched = true
			}
		case ">":
			if key > val {
				matched = true
			}
		case "<=":
			if key <= val {
				matched = true
			}
		case "<":
			if key < val {
				matched = true
			}
		default:
			panic("unsupported op: " + expr.Op)
		}

		if matched {
			return expr.Thens[i]
		}
	}

	return expr.Default.Eval(ctx)
}

type lookupNumExpr struct {
	Key     NumExpression
	Op      string
	Values  []float64
	Thens   []float64
	Default NumExpression
}

func (expr *lookupNumExpr) Eval(ctx *Context) interface{} {
	return expr.EvalNum(ctx)
}

func (expr *lookupNumExpr) EvalNum(ctx *Context) float64 {
	key := expr.Key.EvalNum(ctx)

	for i, val := range expr.Values {
		matched := false
		switch expr.Op {
		case ">=":
			if key >= val {
				matched = true
			}
		case ">":
			if key > val {
				matched = true
			}
		case "<=":
			if key <= val {
				matched = true
			}
		case "<":
			if key < val {
				matched = true
			}
		default:
			panic("unsupported op: " + expr.Op)
		}

		if matched {
			return expr.Thens[i]
		}
	}

	return expr.Default.EvalNum(ctx)
}

type lookupNumExprLTE struct {
	lookupNumExpr
}

func (expr *lookupNumExprLTE) EvalNum(ctx *Context) float64 {
	key := expr.Key.EvalNum(ctx)

	for i, val := range expr.Values {
		if key <= val {
			return expr.Thens[i]
		}
	}

	return expr.Default.EvalNum(ctx)
}

type lookupNumExprGTE struct {
	lookupNumExpr
}

func (expr *lookupNumExprGTE) EvalNum(ctx *Context) float64 {
	key := expr.Key.EvalNum(ctx)

	for i, val := range expr.Values {
		if key >= val {
			return expr.Thens[i]
		}
	}

	return expr.Default.EvalNum(ctx)
}

func compileLookupExpr(expr interface{}) (Expression, error) {
	var err error

	options, ok := expr.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("lookup: must be a hash: (%T, %v)", expr, expr)
	}

	le := &lookupExpr{}

	defaultExpr := options["default"]
	if defaultExpr == nil {
		return nil, errors.Errorf("lookup: must kave default attribute: (%T, %v)", expr, expr)
	}

	le.Default, err = CompileExpression(defaultExpr)
	if err != nil {
		return nil, errors.WithMessage(err, "lookup: default")
	}

	keyExpr := options["key"]
	if keyExpr == nil {
		return nil, errors.Errorf("lookup: must kave key attribute: (%T, %v)", expr, expr)
	}

	le.Key, err = CompileNumExpression(keyExpr)
	if err != nil {
		return nil, errors.WithMessage(err, "lookup: key")
	}

	op := options["op"]
	if op == nil {
		return nil, errors.Errorf("lookup: must kave op attribute: (%T, %v)", expr, expr)
	}

	if o, ok := op.(string); !ok || !stringIn(o, []string{"<", ">", "<=", ">="}) {
		return nil, errors.Errorf("lookup: must kave op must be in ['<', '>', '<=', '>=']: (%T, %v)", expr, expr)
	} else {
		le.Op = o
	}

	// the table
	table := options["table"]
	if table == nil {
		return nil, errors.Errorf("lookup: must kave table attribute: (%T, %v)", expr, expr)
	}

	tables, ok := table.([]interface{})
	if !ok {
		return nil, errors.Errorf("lookup: table attribute must be array: (%T, %v)", expr, expr)
	}

	for i, t := range tables {
		parts, ok := t.([]interface{})
		if !ok || len(parts) != 2 {
			return nil, errors.Errorf("lookup: table element %d must be 2 element array: (%T, %v)", i, expr, expr)
		}

		val, err := parseFloat64(parts[1])
		if err != nil {
			return nil, errors.WithMessage(err, "lookup: value")
		}

		le.Values = append(le.Values, val)
		le.Thens = append(le.Thens, parts[0])
	}

	// see if we can promote to num expression
	return promoteLookupExpr(le), nil
}

func promoteLookupExpr(le *lookupExpr) Expression {
	if !canPromote(le) {
		return le
	}

	lne := &lookupNumExpr{
		Key:     le.Key.(NumExpression),
		Default: le.Default.(NumExpression),
		Values:  le.Values,
		Op:      le.Op,
	}

	for _, t := range le.Thens {
		f, err := parseFloat64(t)
		if err != nil {
			panic("but I could parse the float before?!?")
		}

		lne.Thens = append(lne.Thens, f)
	}

	// For super performance we have even more optimized lookup versions
	// the most common operators.
	switch lne.Op {
	case "<=":
		return &lookupNumExprLTE{*lne}
	case ">=":
		return &lookupNumExprGTE{*lne}
	}

	return lne
}

func canPromote(le *lookupExpr) bool {
	if _, ok := le.Key.(NumExpression); !ok {
		return false
	}

	if le.Default == nil {
		return false
	}

	if _, ok := le.Default.(NumExpression); !ok {
		return false
	}

	for _, t := range le.Thens {
		_, err := parseFloat64(t)
		if err != nil {
			return false
		}
	}

	return true
}
