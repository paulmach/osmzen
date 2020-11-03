package filter

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestBuildingHeight(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		result float64
	}{
		{
			name: "height tag",
			tags: map[string]string{
				"height": "8",
			},
			result: 8.0,
		},
		{
			name: "height from levels",
			tags: map[string]string{
				"building:levels": "4",
			},
			result: 14.0, // 3*4 + 2
		},
		{
			name:   "return 0 if no tags",
			tags:   map[string]string{},
			result: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := buildingHeight(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestEstimateParkingCapacity(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		result float64
	}{
		{
			name: "no capacity",
			tags: map[string]string{
				"amenity": "parking",
			},
			result: 4,
		},
		{
			name: "with capacity tag",
			tags: map[string]string{
				"capacity": "123",
			},
			result: 123.0,
		},
		{
			name: "capacity tag is not a number",
			tags: map[string]string{
				"capacity": "one hundred",
			},
			result: 4,
		},
		{
			name: "number of levels",
			tags: map[string]string{
				"levels": "3",
			},
			result: 3 * 4,
		},
		{
			name: "multi-storey",
			tags: map[string]string{
				"parking": "multi-storey",
			},
			result: 8.0,
		},
	}

	for _, tc := range cases {
		expression, err := compileEstimateParkingCapacity(nil)
		if err != nil {
			panic(err)
		}
		filter := expression.(NumExpression)

		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)
			ctx.area = 4 * 46.0 // results in 4 spaces per level

			v := filter.EvalNum(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestLooksLikeServiceArea(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		result float64
	}{
		{
			name: "no name",
			tags: map[string]string{
				"highway": "service",
			},
			result: 17.0,
		},
		{
			name: "non-matching name",
			tags: map[string]string{
				"highway": "service",
				"name":    "party town",
			},
			result: 17.0,
		},
		{
			name: "ends in service area",
			tags: map[string]string{
				"highway": "service",
				"name":    "something Service Area",
			},
			result: 13.0,
		},
		{
			name: "ends in services",
			tags: map[string]string{
				"highway": "service",
				"name":    "Services",
			},
			result: 13.0,
		},
		{
			name: "ends in travel plaza",
			tags: map[string]string{
				"highway": "service",
				"name":    "something travel plaza",
			},
			result: 13.0,
		},
	}

	for _, tc := range cases {
		expression, err := compileLooksLikeServiceArea(nil)
		if err != nil {
			panic(err)
		}
		filter := expression.(NumExpression)

		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := filter.EvalNum(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}

func TestLooksLikeRestArea(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		result float64
	}{
		{
			name: "no name",
			tags: map[string]string{
				"highway": "rest_area",
			},
			result: 17.0,
		},
		{
			name: "non-matching name",
			tags: map[string]string{
				"highway": "rest_area",
				"name":    "party town",
			},
			result: 17.0,
		},
		{
			name: "ends in rest area",
			tags: map[string]string{
				"highway": "rest_area",
				"name":    "something Rest Area",
			},
			result: 13.0,
		},
	}

	for _, tc := range cases {
		expression, err := compileLooksLikeRestArea(nil)
		if err != nil {
			panic(err)
		}
		filter := expression.(NumExpression)

		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties["tags"] = tc.tags
			ctx := NewContext(nil, f)

			v := filter.EvalNum(ctx)
			if v != tc.result {
				t.Errorf("wrong result: %v != %v", v, tc.result)
			}
		})
	}
}
