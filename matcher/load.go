package matcher

import (
	"encoding/csv"
	"io"
)

// Load will create a matcher from the csv file.
func Load(r io.Reader) (*Matcher, error) {
	csvr := csv.NewReader(r)
	records, err := csvr.ReadAll()
	if err != nil {
		return nil, err
	}

	return Compile(records[0], records[1:])
}
