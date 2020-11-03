package transform

import (
	"testing"

	"github.com/paulmach/orb/geojson"
)

func TestPoisCapacity(t *testing.T) {
	cases := []struct {
		name       string
		properties map[string]interface{}
		result     float64
	}{
		{
			name: "should floor if number",
			properties: map[string]interface{}{
				"capacity": 123.987,
			},
			result: 123.0,
		},
		{
			name: "should convert and floor a string number",
			properties: map[string]interface{}{
				"capacity": "123.987",
			},
			result: 123.0,
		},
		{
			name: "should be null if not a number",
			properties: map[string]interface{}{
				"capacity": "one hundred",
			},
			result: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := geojson.NewFeature(nil)
			f.Properties = tc.properties

			poisCapacity(nil, f)
			v := f.Properties["capacity"]
			if v == nil {
				if tc.result != -1 {
					t.Errorf("expected null got %v", v)
				}
			} else {
				// must be float64 if not null
				c := v.(float64)
				if c != tc.result {
					t.Errorf("wrong result: %v != %v", c, tc.result)
				}
			}
		})
	}
}
