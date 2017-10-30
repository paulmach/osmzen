package transform

import "testing"

func TestRoadShieldText(t *testing.T) {
	cases := []struct {
		name         string
		network, ref string
		result       string
	}{
		{
			name: "empty",
		},
		{
			name:   "special FI-PI-LI",
			ref:    "FI-PI-LI",
			result: "FI-PI-LI",
		},
		{
			name:    "starts with ua and matches",
			network: "ua:xyz",
			ref:     "a-10-10",
			result:  "a1010",
		},
		{
			name:    "starts with ua and not matches",
			network: "ua:xyz",
			ref:     "10-12",
			result:  "10",
		},
		{
			name:    "gr network returns ref",
			network: "gr:xyz",
			ref:     "10-12",
			result:  "10-12",
		},
		{
			name:    "GR network returns ref",
			network: "GR:xyz",
			ref:     "10-12",
			result:  "10-12",
		},
		{
			name:    "number at front",
			network: "xyz",
			ref:     "10-abx",
			result:  "10",
		},
		{
			name:    "letter than numbers",
			network: "xyz",
			ref:     "ca 6",
			result:  "6",
		},
		{
			name:    "long letter than numbers",
			network: "xyz",
			ref:     "ca 6;ca 20",
			result:  "6",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := roadShieldText(tc.network, tc.ref)
			if r != tc.result {
				t.Errorf("incorrect shield: %v != %v", r, tc.result)
			}
		})
	}
}
