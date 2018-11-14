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
