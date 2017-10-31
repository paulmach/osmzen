osmzen [![Build Status](https://travis-ci.org/paulmach/osmzen.png?branch=master)](https://travis-ci.org/paulmach/osmzen) [![Godoc Reference](https://godoc.org/github.com/paulmach/osmzen?status.png)](https://godoc.org/github.com/paulmach/osmzen)
======

This is a port of [tilezen/vector-datasource](https://github.com/tilezen/vector-datasource) developed by
[Mapzen](https://mapzen.com/). It converts [Open Street Map](https://www.openstreetmap.org/) data
directly into GeoJSON with properties that are understood by [Mapzen house
styles](https://mapzen.com/products/maps/).


A Postgres database is not required to evaluate the logic that is originally defined in a combination
of SQL and Python. This allows for the quick mapping of any OSM element(s) to a `kind`/`kind_detail`
normalization. Such a normalization is non-trivial given the "diversity" of OSM tagging so projects
like tilezen/vector-datasource (and may others) are necessary.

The port currently implements almost all features applicable to evaluating zoom 14+ tile data.
These features include:

* all filter, min_zoom and output logic defined in the `yaml/*.yaml` files,
* all transforms that apply, implementation specific data transforms are skipped,
* the CSV matcher post processor to set the `scale_rank` and `sort_rank` properties,
* geometry clipping and label placement logic.

A lot of post processors still need to be ported, but only a few of the missing ones apply
to zooms 14+. Missing post processors include: landuse_kind intercuts, merging line strings
and merging building with building parts.

It would also be nice to port some of the integration tests as they would give confidence that
things are really working as expected. Right now there are just some unit tests and some
high level sanity checks.

#### Changes from the original tilezen/vector-datasource

The goal is for there to be no functional differences for zooms 14+. The YAML definition files are
unchanged, there a just a few minor changes to the post processor filtering in `queries.yaml`. See
the [github diff](https://github.com/tilezen/vector-datasource/compare/master...paulmach:master).

The port is based off of [v1.4.0ish](https://github.com/tilezen/vector-datasource/releases/tag/v1.4.0)
version of the vector-datasource. The [fork](https://github.com/paulmach/vector-datasource) or the
[github diff](https://github.com/paulmach/vector-datasource/compare/master...tilezen:master) between
it and upstream/master are kept at the intended "reference".

Usage
-----

1. Load and compile the `queries.yaml`, `yaml/*.yaml` and `spreadsheets/*_rank/*.csv` files. This can
	be done by loading the files directly using the implied directory structure:

		config, err := osmzen.Load("config/queries.yaml")

	or if you want to use the "official" ported config files but don't want to distribute them with
	the binary you can make use of the `embeddedconfig` subpackage which uses
	[go-bindata](https://github.com/jteeuwen/go-bindata) to "compile in" the files:

		config, err := osmzen.LoadEmbeddedConfig(embeddedconfig.Asset)

	If there are mistakes in the YAML the error will contain a lot of information to help debug:

		if err, ok := errors.Cause(err).(*filter.CompileError); ok {
			log.Printf("error: %v", err.Error())
			log.Printf("cause: %v", err.Cause)
			log.Printf("yaml:\n%s", err.YAML()) // chunk of marshalled YAML with the issue
		} else if err != nil {
			log.Printf("other err: %v", err)
		}

2. Process some OSM data:

		data := osm.OSM{}
		layers, err := config.Process(data, geo.Bound(-180, 180, -90, 90), zoom)

		// layers is defined as `map[string]*geojson.FeatureCollection`

	Layers can also be processed individually:

		featureCollection, err := config.Layers["buildings"].Process(data, zoom)

The result is a GeoJSON feature collection with `kind`, `kind_detail` etc. properties that
are understood by [Mapzen house styles](https://mapzen.com/products/maps/).

## Example

A more complete example that loads a zoom 16 area from the OSM API and
the processes the tile (minus error checking):

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/paulmach/osmzen"
	"github.com/paulmach/osmzen/embeddedconfig"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmapi"
)

func main() {
	tile := maptile.New(19613, 29310, 16)

	// load osmzen config
	config, _ := osmzen.LoadEmbeddedConfig(embeddedconfig.Asset)

	// get osm data for a tile from the offical api.
	bounds, _ := osm.NewBoundsFromTile(tile)
	data, _ := osmapi.Map(context.Background(), bounds)

	// process the data
	// The tile coords will be used to exclude include interesting nodes
	// and labels outside the tile.
	layers, _ := config.Process(data, tile.Bound(), tile.Z)

	// pretty print the json
	pretty, _ := json.MarshalIndent(layers, "", " ")
	fmt.Println(string(pretty))
}
```

Implementation details
----------------------

At a high level [tilezen/vector-datasource](https://github.com/tilezen/vector-datasource) filters and
process's its data using the following steps:

1. find relevant elements for a layer using the SQL query defined in `data/{layer_name}.jinja`,
2. filter the elements using filter *conditions* defined in `yaml/{layer_name}.yaml`,
3. generate properties for each element using the matching filter's output *expressions*,
4. apply *transforms* to each element independently,
5. apply *post processes* to all the layers together.

The transforms and post processes that apply to each layer and zoom are defined in `queries.yaml`.
For a lot more details see the official tilezen/vector-datasource [project
overview](https://github.com/tilezen/vector-datasource/blob/master/CONTRIBUTING.md).

As this package is a port of that code it follows the same steps, except for step 1 since the data
is passed in directly.

### Loading and compiling config

During the loading of the YAML+CSV config files everything is compiled to make sure all the
expressions and function references are known. If there is a typo, or something new/unsupported, an
error will be returned. See above for how to get useful information from the error. The initial
compile step allows for the checking of config errors at startup. Also since the types are converted
up front there is a nice performance boost of about 10x.

The filters and outputs defined in the `yaml/*.yaml` files are basically a set of statements that
act like: "if the element tags look like this, output these kind, kind_detail, etc. properties".

The filters define a condition, yes/no matching, that evaluates into a boolean. During the compile
step these are converted into concrete types that implement the `filter.Condition` interface. The
interface is defined as:

	type filter.Condition interface {
		Eval(*filter.Context) bool
	}

The output for each filter defines what properties should be assigned to the element's GeoJSON
feature. They output things such as booleans (is_tunnel), strings (kind), numbers (area) or nil to
be ignored. The interface is defined as:

	type fitler.Expression interface {
		Eval(*filter.Context) interface{}
	}

	type filter.NumExpression interface {
		filter.Expression
		EvalNum(*filter.Context) float64
	}

The `filter.NumExpression` is also implemented by expressions that must be a number (e.g. area,
building height). Using it helps avoid a type indirection when we know we need numbers. For example
the `min` and `max` expressions.

The `filter.Context` is passed in at runtime and contains info about the element being evaluated
like the OSM tags and geometry. It also caches "expensive" things like the area and volume that can
be used by multiple filters.

#### Transforms and post processes

After elements for a layer are matched and GeoJSON features are created, a set of transforms is
applied. The transforms edit the element properties based on some logic, sometimes requiring the
set of relations the original OSM element is a member of.

The **transforms** are matched while loading the config to a function of the form:

	func(*filter.Context, *geojson.Feature)

Transforms can just change a feature, they can't remove a feature if it's "bad" for any reason, like
too small for the zoom. Transforms also don't know about other features so they can't be used to
remove duplicates or merge features, like parts of the same road. However, transforms can be used to
do things like fix one-way direction, add the correct highway shield text, abbreviate road names,
etc.

The **post processes** are compiled to load files and check the parameters. They are mapped to an
object implementing the `postprocess.Function` interface defined as:

	type postprocess.Function interface {
		Eval(*postprocess.Context, map[string]*geojson.FeatureCollection)
	}

The function takes all the layers as input. Some examples of post processing are clipping to the
tile bounds, setting sort_rank and scale_rank, removing duplicate features, removing small areas,
merging lines, etc.

### Evaluating some data

Once everything is all setup we can start evaluating data against the filters and apply the
transforms and post processes. The input is OSM data, a bound, plus a zoom. The bound is used to
clip geometry and check if a label should be included. The zoom is used to filter out
things that are "too small" as defined by the `min_zoom` output in the `yaml/*.yaml` files. To
include everything, use a high zoom, such as 20.

The evaluation proceeds in the following steps:

1. Convert OSM data to GeoJSON

	The data is run through [osm/osmgeojson](https://github.com/paulmach/osm/tree/master/osmgeojson)
	which is a port of the [osmtogeojson](https://github.com/tyrasd/osmtogeojson) node.js library.
	This groups nodes into ways and ways into polygons. For example, we don't care about the 4 nodes
	that define a building, we just want the building polygon.

2. Run each OSM element GeoJSON feature through the filters

	We find the first filter in each layer to match and then compute the filter's outputs. Note,
	that an element can match in multiple layers, for example a building polygon and a POI.
	The input and output are both GeoJSON, however, the input contains properties based on OSM tags,
	but the output has properties from the filter like the `kind` and `kind_detail` etc.

3. Apply the transforms

	The new GeoJSON object is updated a bit. This can include reversing the geometry or simplifying
	the name.

4. Apply the post processes to all the layers.

The end result is a layer, or set of layers that match those produced by `tilezen`.
Note that this whole process can be applied to a single element.

### Benchmarks

The first two benchmarks evaluate a single element against ALL the filters and outputs
in that layer. Normally you can stop after the first match and only evaluate that one output.
The third benchmark is more typical of normal usage and coverts data from a zoom 16 tile.
The last benchmark leaves out the osm data to GeoJSON step and just does the filtering
and processing unique to this package.

```
BenchmarkBuildings-4      200000        9969 ns/op       1040 B/op       42 allocs/op
BenchmarkPOIs-4            10000      171457 ns/op       6816 B/op      450 allocs/op
BenchmarkFullTile-4          100    11292314 ns/op    3611916 B/op    26555 allocs/op
BenchmarkProcessGeoJSON-4    200     8091129 ns/op    1978560 B/op    18319 allocs/op
```

These benchmarks were run on a 2017 MacBook Pro with a 3.1 ghz processor and 8 gigs of ram.
No concurrency is used in this package.

#### This library makes use of the following packages:

* [github.com/pkg/errors](https://github.com/pkg/errors) - for rich errors with stack traces
* [gopkg.in/yaml.v2](http://gopkg.in/yaml.v2) - YAML parsing
* [github.com/paulmach/orb](https://github.com/paulmach/orb) - geometry area, centroid, clipping, etc.
* [github.com/paulmach/osm](https://github.com/paulmach/osm)
