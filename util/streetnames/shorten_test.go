package streetnames

import "testing"

func TestShorten(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"Wodehouse Avenue", "Wodehouse Ave."},
		{"Wodehouse Rd", "Wodehouse Rd."},
		{"Wodehouse Street North", "Wodehouse St. N"},
		{"South Wodehouse Boulevard", "S Wodehouse Blvd."},
		{"East Highway", "East Hwy."},
		{"North Expressway Northeast", "North Expwy. NE"},
		{"SW North Lane", "SW North Ln."},
		{"Street Dr", "Street Dr."},
		{"Road Street North", "Road St. N"},
		{"Parkway", "Parkway"},
		{"   East   Highway   ", "East Hwy."},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			r := Shorten(tc.in)
			if r != tc.out {
				t.Errorf("incorrect shorten: %v != %v", r, tc.out)
			}
		})
	}
}
