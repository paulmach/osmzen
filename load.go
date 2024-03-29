package osmzen

import (
	"embed"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/paulmach/osmzen/filter"
	"github.com/paulmach/osmzen/postprocess"
	"github.com/paulmach/osmzen/transform"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// DefaultConfig is the reference to yaml and csv files with the
// default configuration of kind/kind_detail mappings. These
// files are found in the config sub-directory.
//go:embed config
var DefaultConfig embed.FS

type ReadFiler interface {
	ReadFile(string) ([]byte, error)
}

// Config is the full queries.yaml config file for this library.
type Config struct {
	All         []string              `yaml:"all"`
	Layers      map[string]*Layer     `yaml:"layers"`
	PostProcess []*postprocess.Config `yaml:"post_process"`

	postProcessors []postprocess.Function
	clipFactors    map[string]float64
}

// Layer defines config for a single layer.
type Layer struct {
	ClipFactor    float64  `yaml:"clip_factor"`
	GeometryTypes []string `yaml:"geometry_types"`
	Transforms    []string `yaml:"transform"`

	// Currently unused
	Sort                   string `yaml:"sort"`
	AreaInclusionThreshold int    `yaml:"area-inclusion-threshold"`

	filters    []*filter.Filter
	transforms []transform.Transform
}

// Load take a path to the queries.yaml file and load+compiles it.
func Load(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read config")
	}

	dir, _ := path.Split(filename)
	return loadConfig(data, func(name string) ([]byte, error) {
		return os.ReadFile(path.Join(dir, name))
	})
}

// LoadEmbeddedConfig will load the config and layers using the compiled in assets.
//
// Deprecated: use LoadDefaultConfig instead
func LoadEmbeddedConfig(asset func(string) ([]byte, error)) (*Config, error) {
	data, err := asset("queries.yaml")
	if err != nil {
		return nil, err
	}

	return loadConfig(data, asset)
}

// LoadDefaultConfig will load the default config embedded in this package.
func LoadDefaultConfig() (*Config, error) {
	data, err := DefaultConfig.ReadFile("config/queries.yaml")
	if err != nil {
		return nil, err
	}

	return loadConfig(data, func(name string) ([]byte, error) {
		return DefaultConfig.ReadFile("config/" + name)
	})
}

func loadConfig(data []byte, asset func(name string) ([]byte, error)) (*Config, error) {
	c := &Config{}
	err := yaml.Unmarshal(data, &c)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to unmarshal")
	}

	// clips factors is one, of potentially many things defined on the
	// layer config that is needed by the post processors. All the information
	// needs to be found here and passed to the compilers.
	c.clipFactors = make(map[string]float64)
	for _, name := range c.All {
		lc := c.Layers[name]
		err := lc.load(name, asset)
		if err != nil {
			return nil, errors.WithMessage(err, name)
		}

		c.clipFactors[name] = lc.ClipFactor
	}

	ppctx := &postprocess.CompileContext{
		Asset:       asset,
		ClipFactors: c.clipFactors,
	}
	for i, p := range c.PostProcess {
		f, err := postprocess.Compile(ppctx, p)
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("post process %d", i))
		}

		if f == nil {
			continue
		}

		c.postProcessors = append(c.postProcessors, f)
	}

	return c, nil
}

func (l *Layer) load(name string, asset func(string) ([]byte, error)) error {
	if l == nil {
		return errors.Errorf("undefined layer")
	}

	data, err := asset(path.Join("yaml", name+".yaml"))
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to load %v", name))
	}

	l.filters, err = layerCompile(data)
	if err != nil {
		return err
	}

	l.transforms = make([]transform.Transform, 0, len(l.Transforms))
	for _, t := range l.Transforms {
		tf, ok := transform.Map(t)
		if !ok {
			return errors.Errorf("transform undefined: %s", t)
		}

		if tf != nil {
			l.transforms = append(l.transforms, tf)
		}
	}

	return nil
}

func layerCompile(data []byte) ([]*filter.Filter, error) {
	l := &struct {
		Filters []*filter.Filter `yaml:"filters"`
	}{}

	err := yaml.Unmarshal(data, &l)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to unmarshal")
	}

	for i, f := range l.Filters {
		if err := f.Compile(); err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("failed on filter %d", i))
		}
	}

	return l.Filters, nil
}
