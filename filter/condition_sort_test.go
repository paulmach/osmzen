package filter

import "testing"

func TestConditionSort(t *testing.T) {
	conds := []Condition{
		&volumeCond{},
		&allCond{},
		&stringCond{},
	}

	conditionSort(conds)

	if _, ok := conds[0].(*stringCond); !ok {
		t.Errorf("incorrect condition: %T", conds[0])
	}

	if _, ok := conds[1].(*allCond); !ok {
		t.Errorf("incorrect condition: %T", conds[1])
	}

	if _, ok := conds[2].(*volumeCond); !ok {
		t.Errorf("incorrect condition: %T", conds[2])
	}
}
