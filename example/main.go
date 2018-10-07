package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/paulmach/osmzen"
	"github.com/paulmach/osmzen/embeddedconfig"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmapi"

	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/maptile"
)

// Port to serve on, if you want to change it
const Port = "8100"

func main() {
	// load and initialize the mapzen context using the default config files
	config, err := osmzen.LoadEmbeddedConfig(embeddedconfig.Asset)
	if err != nil {
		panic(err)
	}

	// handler to return the index HTML file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadFile("index.html")
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		w.Write(data)
	})

	// handler to serve the tiles
	http.HandleFunc("/tiles/", func(w http.ResponseWriter, r *http.Request) {
		tile := parsePath(r.URL.Path) // find tile bounds for request
		bound, _ := osm.NewBoundsFromTile(tile)

		// get the osm data for that bound
		data, err := osmapi.Map(r.Context(), bound)
		if err != nil {
			if err := r.Context().Err(); err != nil {
				// what if not "context canceled"?
				http.Error(w, err.Error(), 500)
				return
			}

			panic(err)
		}

		// Process the data into mapzen vector tiles format
		layers, err := config.Process(
			data,                            // osm data
			geo.BoundPad(tile.Bound(), 100), // clip the geometries to this bound, add 100 meters of padding.
			tile.Z,                          // zoom, used to leave out things when zoomed out. Doesn't do much in this context.
		)
		if err != nil {
			w.Write([]byte(err.Error()))
			panic(err)
		}

		json.NewEncoder(w).Encode(layers)
	})

	log.Printf("Starting Demo server, open http://localhost:%s", Port)
	log.Fatal(http.ListenAndServe("localhost:"+Port, nil))
}

// parsePath converts the `/tiles/{z}/{x}/{y}.json` path into a tile.
func parsePath(p string) maptile.Tile {
	parts := strings.Split(p, "/")

	return maptile.Tile{
		X: parseNum(parts[3]),
		Y: parseNum(parts[4]),
		Z: maptile.Zoom(parseNum(parts[2])),
	}
}

func parseNum(n string) uint32 {
	parts := strings.Split(n, ".")
	v, _ := strconv.Atoi(parts[0])
	return uint32(v)
}
