osmzen tile server
==================

![tile server in action](screenshot.png?raw=true)

This example provides a basic [OSM](https://www.openstreetmap.org/) tile server that:

1. reads data for a time from the [OSM API](https://wiki.openstreetmap.org/wiki/API_v0.6#Retrieving_map_data_by_bounding_box:_GET_.2Fapi.2F0.6.2Fmap),
2. converts it into [Mapzen Vector Tiles](https://mapzen.com/documentation/vector-tiles/layers/) using this package,
3. serves it up to be displayed by [Tangram](https://mapzen.com/products/tangram/) in the browser.

### Setup

Go version 1.9+ is required. Must be installed first. Then run the following commands in the console:

	# install this code into your GOPATH
	go get -u github.com/paulmach/osmzen

	# navigate to the installed directory
	cd $GOPATH/src/github.com/paulmach/osmzen/example

	# run the server
	go run main.go

	# open the page in the browser
	[http://localhost:8100](http://localhost:8100)
