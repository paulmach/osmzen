package ranker

import (
	"fmt"
	"io/ioutil"

	"github.com/paulmach/orb/geojson"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// Ranker will evaluate feature properties against the structure of the
// collision_rank.yaml file.
type Ranker struct {
	// matchers are hashed using layer+kind keys
	matchers map[string]map[string][]*matcher
	catchAll int
}

type matcher struct {
	cond Condition
	rank int
}

// LoadFile loads/creates a ranking using the yaml data located at
// the give filename.
func LoadFile(filename string) (*Ranker, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return Load(data)
}

// Load will preprocess the data into a ranker object.
func Load(data []byte) (*Ranker, error) {
	ranks := []map[string]interface{}{}
	err := yaml.Unmarshal(data, &ranks)
	if err != nil {
		return nil, err
	}

	ranker := &Ranker{
		matchers: make(map[string]map[string][]*matcher),
	}

	index := 1
	for _, rank := range ranks {
		if r, ok := rank["_reserved"].(map[interface{}]interface{}); ok {
			if c, ok := r["count"].(int); ok {
				index += c
				continue
			}

			from, ok1 := r["from"].(int)
			to, ok2 := r["to"].(int)
			if !ok1 || !ok2 {
				panic("ranker: reserved from and to values must be integers")
			}

			if from < index {
				panic(fmt.Sprintf("ranker: reserved index %d already use, wanted to reserve from %d", index, from))
			}

			index = to + 1
			continue
		}

		layer, ok := rank["$layer"].(string)
		if !ok {
			if _, ok := rank["$layer"].(bool); ok {
				ranker.catchAll = index
				break
			}
			return nil, errors.Errorf("ranker: $layer require and must be string: %T %v", rank, rank)
		}
		if ranker.matchers[layer] == nil {
			ranker.matchers[layer] = make(map[string][]*matcher)
		}

		matcher, err := makeMatcher(rank, index)
		if err != nil {
			// TODO: maybe better error reporting
			return nil, err
		}

		kind, _ := rank["kind"].(string) // if no kind index by ""
		ranker.matchers[layer][kind] = append(ranker.matchers[layer][kind], matcher)
		index++
	}

	return ranker, nil
}

// Rank will compute the rank for the give layer and feature properties.
func (r *Ranker) Rank(layerName string, props geojson.Properties) int {
	layer, ok := r.matchers[layerName]
	if !ok {
		return r.catchAll
	}

	matchers, ok := layer[props.MustString("kind", "")]
	if !ok {
		return r.catchAll
	}
	matchers = append(matchers, layer[""]...) // include matchers with no kind

	for _, m := range matchers {
		if m.Eval(props) {
			return m.rank
		}
	}

	return r.catchAll
}

func (m *matcher) Eval(props geojson.Properties) bool {
	if m.cond == nil {
		return true
	}

	return m.cond.Eval(props)
}

func makeMatcher(props geojson.Properties, rank int) (*matcher, error) {
	np := make(map[interface{}]interface{})
	for k, v := range props {
		if k == "$layer" {
			continue
		}
		if k == "kind" {
			continue
		}
		np[k] = v
	}

	if len(np) == 0 {
		return &matcher{rank: rank}, nil
	}

	cond, err := CompileCondition(np)
	if err != nil {
		return nil, err
	}

	return &matcher{
		cond: cond,
		rank: rank,
	}, nil
}
