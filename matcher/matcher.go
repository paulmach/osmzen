package matcher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/paulmach/orb/geo/geojson"
	"github.com/pkg/errors"
)

// A Matcher represents the spreadsheet conditions for sort_rank and scale_rank.
type Matcher struct {
	outputKey  string
	properties []string
	rows       []*row
}

// A Context provides some extra information about what is being matched.
// Should contain information that is shared for a complete request.
type Context struct {
	Zoom float64
}

// Compile will take a list of the headers and the rows and compile
// it into a matcher.
func Compile(headers []string, rows [][]string) (*Matcher, error) {
	m := &Matcher{
		outputKey:  headers[len(headers)-1],
		properties: make([]string, len(headers)-1),
		rows:       make([]*row, len(rows)),
	}

	headers = headers[:len(headers)-1] // last is the output key
	for i, h := range headers {
		m.properties[i] = strings.Split(h, "::")[0]
	}

	for i, r := range rows {
		cr, err := compileRow(r)
		if err != nil {
			return nil, err
		}

		m.rows[i] = cr
	}

	return m, nil
}

// Eval will evaluate the matcher for the given geojson feature.
// If there is a match it'll add the output property to the feature.
func (m *Matcher) Eval(ctx *Context, feature *geojson.Feature) bool {
	props := make([]interface{}, len(m.properties))
	for i := range props {
		switch m.properties[i] {
		case "zoom":
			props[i] = ctx.Zoom
		default:
			props[i] = feature.Properties[m.properties[i]]
		}
	}

	for _, r := range m.rows {
		if v, ok := r.Eval(props); ok {
			feature.Properties[m.outputKey] = v
			return true
		}
	}

	return false
}

type row struct {
	Value float64
	Cells []cell
}

func (r *row) Eval(props []interface{}) (float64, bool) {
	for i := range r.Cells {
		if !r.Cells[i].Eval(props[i]) {
			return 0, false
		}
	}

	return r.Value, true
}

func compileRow(columns []string) (*row, error) {
	if len(columns) < 2 {
		return nil, errors.New("matchers: need at least two columns")
	}

	val, err := strconv.ParseFloat(columns[len(columns)-1], 32)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse final value")
	}

	r := &row{
		Value: val,
		Cells: make([]cell, len(columns)-1),
	}

	for i := 0; i < len(columns)-1; i++ {
		c, err := compileCell(columns[i])
		if err != nil {
			return nil, err
		}

		if c != nil {
			r.Cells[i] = c
		}
	}

	return r, nil
}

type cell interface {
	Eval(val interface{}) bool
}

func compileCell(c string) (cell, error) {
	if c == "*" {
		return anyCell{}, nil
	} else if c == "-" {
		return noneCell{}, nil
	} else if c == "+" {
		return someCell{}, nil
	} else if c == "true" {
		return trueCell{}, nil
	} else if strings.Contains(c, ";") {
		return setCell{strings.Split(c, ";")}, nil
	} else if strings.HasPrefix(c, ">=") {
		v, err := strconv.ParseFloat(c[2:], 32)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid matcher: %v", c)
		}

		return greaterThanEqualCell{v}, nil
	} else if strings.HasPrefix(c, "<=") {
		v, err := strconv.ParseFloat(c[2:], 32)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid matcher: %v", c)
		}

		return lessThanEqualCell{v}, nil
	} else if strings.HasPrefix(c, ">") {
		v, err := strconv.ParseFloat(c[2:], 32)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid matcher: %v", c)
		}

		return greaterThanCell{v}, nil
	} else if strings.HasPrefix(c, "<") {
		v, err := strconv.ParseFloat(c[2:], 32)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid matcher: %v", c)
		}

		return lessThanCell{v}, nil
	} else if strings.HasPrefix(c, "!") {
		return nil, fmt.Errorf("invalid matcher: %v", c)
	} else {
		f, err := strconv.ParseFloat(c, 64)
		if err == nil {
			return exactFloat64Cell{f, c}, nil
		}
		return exactCell{c}, nil
	}
}

type anyCell struct{}

func (ac anyCell) Eval(val interface{}) bool {
	return true
}

type noneCell struct{}

func (nc noneCell) Eval(val interface{}) bool {
	return val == nil
}

type someCell struct{}

func (sc someCell) Eval(val interface{}) bool {
	return val != nil
}

type trueCell struct{}

func (tc trueCell) Eval(val interface{}) bool {
	v, ok := val.(bool)
	return ok && v
}

type setCell struct {
	Vals []string
}

func (sc setCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	s := val.(string)
	for _, v := range sc.Vals {
		if v == s {
			return true
		}
	}

	return false
}

type greaterThanEqualCell struct {
	Val float64
}

func (c greaterThanEqualCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	return val.(float64) >= c.Val
}

type greaterThanCell struct {
	Val float64
}

func (c greaterThanCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	return val.(float64) > c.Val
}

type lessThanEqualCell struct {
	Val float64
}

func (c lessThanEqualCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	return val.(float64) <= c.Val
}

type lessThanCell struct {
	Val float64
}

func (c lessThanCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	return val.(float64) < c.Val
}

type exactFloat64Cell struct {
	Val    float64
	String string
}

func (c exactFloat64Cell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}

	switch val := val.(type) {
	case float64:
		return c.Val == val
	case int:
		return c.Val == float64(val)
	}

	return val.(string) == c.String
}

type exactCell struct {
	Val string
}

func (c exactCell) Eval(val interface{}) bool {
	if val == nil {
		return false
	}
	return val.(string) == c.Val
}
