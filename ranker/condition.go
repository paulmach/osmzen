package ranker

import "github.com/pkg/errors"

// Condition is a matcher in the YAML file that can be evaluated.
// It is not as full featured as filter.Condition but differs in that
// it takes a string->interface{} map vs. a string->string.
type Condition interface {
	Eval(map[string]interface{}) bool
}

// CompileCondition compiles the matcher in the collision_rank YAML file so
// it can be evaluated. YAML will decompile hashes into this map with
// interface{} keys. We assume they are always strings.
func CompileCondition(cond map[interface{}]interface{}) (Condition, error) {
	conds := []Condition{}
	for key, val := range cond {
		switch key {
		case "not":
			c, err := CompileCondition(val.(map[interface{}]interface{}))
			if err != nil {
				return nil, err
			}

			conds = append(conds, &notCond{cond: c})
		default:
			// we want to only support comparable types. If the file adds
			// nested hashes we need to do more work.
			if _, ok := val.(map[interface{}]interface{}); ok {
				return nil, errors.Errorf("compile: key %v is a hash", key)
			}

			conds = append(conds, &eqCond{
				Key: key.(string),
				Val: val,
			})
		}
	}

	if len(conds) == 1 {
		return conds[0], nil
	}

	return &allCond{conds: conds}, nil
}

type eqCond struct {
	Key string
	Val interface{}
}

func (c *eqCond) Eval(vals map[string]interface{}) bool {
	return vals[c.Key] == c.Val
}

type notCond struct {
	cond Condition
}

func (c *notCond) Eval(vals map[string]interface{}) bool {
	return !c.cond.Eval(vals)
}

type allCond struct {
	conds []Condition
}

func (c *allCond) Eval(vals map[string]interface{}) bool {
	for _, c := range c.conds {
		if !c.Eval(vals) {
			return false
		}
	}

	return true
}
