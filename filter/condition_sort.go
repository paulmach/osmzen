package filter

import (
	"sort"
)

// conditionSort attempts to keep a consistent order of the conditions
// in any or all condition.
func conditionSort(conds []Condition) {
	sort.Slice(conds, func(i, j int) bool {
		return condSortVal(conds[i]) < condSortVal(conds[j])
	})
}

func condSortVal(c Condition) int {
	// Currently ordered by what I think is the cheapest to evaluate.
	// In the future could consider how often then fail to short circuit the any/all conditions.
	switch c.(type) {
	case *stringCond: // requires map lookup
		return 0
	case *boolCond: // requires map lookup
		return 1
	case *stringInCond: // requires map lookup
		return 5
	case *notCond: // usually string or stringIn so require map lookup.
		return 5
	case *geometryTypesCond:
		return 5
	case *allCond:
		return 10
	case *anyCond:
		return 10
	case *osmTagsCond:
		return 10
	case *compareCond:
		return 15
	case *wayAreaCond:
		return 20
	case *volumeCond:
		return 20
	}

	return 0
}
