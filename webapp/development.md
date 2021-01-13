# Webapp Construction and Design
Webapp is a go application designed to operate a web server for purposes of visualizing and testing the SCION infrastructure. Webapp occupies a strange place in the SCIONLab ecosystem, in that, it draws from a wide variety of sources to provide testing and visualization features so a list of [dependencies](dependencies.md) has been developed for maintenance purposes. There isn't one central source or API for the information webapp uses to interrogate SCIONLab, thus webapp may do the following:

* Read from environment variables.
* Scan SCION's logs.
* Scan SCION's directory structure.
* Call third-party service APIs.
* Request static configuration from a SCIONLab-maintained location.
* Execute bash scripts.
* Execute SCION or SCIONLab tools and apps.
* Read from SCION's databases.
* Make connections to SCION services, like the SCION Daemon.

## Users
* **SCIONLab AS Operators**: Visualize and test the SCIONLab production network.
* **Developers**: Visualize and test any topology you create on `localhost`.

## Structure
* `lib/` - go modules governing sciond, images, health checks, command-line app parsing
* `models/` - go webapp database schema and CRUD operations
* `tests/` - scripts and offline versions of data retrieval for testing
* `util/` - go logging and other universally useful code
* `web/` - root of the static website to serve
    * `config/` - default addresses of command-line apps, also locally saved user settings
    * `data/` - location of image and long term app responses *(generated)*
    * `logs/` - location of webapp log *(generated)*
    * `static/` - location of static code to serve, custom and third-party
        * `css/` - custom style is mostly `style.css`, `style-viz.css`, `topology.css`
        * `js/` - JavaScript
            * `webapp.js` - main JavaScript for the Apps menu that governs real-time graphs and paths configuration
            * `tab-paths.js` - controls topology/map switch, parsing of retrieved paths/locations/labels
            * `tab-topocola.js` - operates the path topology graph, nodes, arrows
            * `tab-g-maps.js` - operates the path map graph along with `web/static/html/map.html`
            * `asviz.js` - operates AS topology graph, path selection
            * `topology.js` - utility functions to sort and order paths data
            * `location.js` - utility functions manage maps
        * `html/` - non-template HTML, mostly `map.html` that is injected into a frame
    * `webapp.db` - SQL Lite database *(generated)* of command-line app responses, short term 24-hour lifetime.
    * `template/` - most of the site HTML, broken into pages per navbar menu
    * `tests/health/` - bash scripts and json config to run health checks
* `webapp.go` - go main module for webapp, governing web-server, AJAX-style requests, and executing command-line apps.

## Website Features
1. **Launch Config**: Several command line parameters allow you to define multiple locations where your SCION deployment is running and where this website should be served from and to.
1. **Health Checks**: The default first page in the Health Checks that will run several bash scripts checking for common SCION misconfiguration and operational requirements.
1. **AS Address (IA)**: On a local development topology you can switch perspective to any AS in the topology from the Navbar. Unless your production deployment co-locates multiple ASes, in production usually only one AS will be available.
1. **Apps Tests**: On the Apps menu, run test for bandwidth, latency, routing, and IoT sensors at ETH (camera, temperature, humidity, ambient noise, etc.). Many of these tests can be run continuously and graphs are provided to show performance over time. Expanding the available paths tree in many cases will select the path to run a test app on.
1. **Paths Visualization**: A graph is provided by default to visualize available paths to the user's AS, and to allow visual examination of individual path routes through the SCION instrstrcutre.

## Future Features
* **Latency Tests**: *(in development)* SCION ping and traceroute results would be listed next to available paths or overlaid on path graphs allowing easy examination of performance.
* **Name Resolution**: Integration with the SCION name resolution service (RAINS), would optionally show human-readable hostnames rather than IA numbers.
* **Geolocation**: Dynamic locations provided by ASes issuing beacons with geolocation would be more accurate and complete than the current static geolocation data on the global paths map.
* **Web sockets**: Data between the go web-server and the browser is requested by AJAX and polling from the browser. Implementing websockets would avoid timing issues polling for updated graph data from the web-server and reduce browser load.

## Open Source Code Used
* [Highcharts](https://www.highcharts.com/): Apps menu linear graph rendering.
* [D3](https://d3js.org/) and [Cola](https://ialab.it.monash.edu/webcola/): Apps menu path topology graph and Monitor menu AS topology graph.
* [Bootstrap](https://getbootstrap.com/): Some tabs styling and icons.
* [Google Maps](https://developers.google.com/maps/documentation) API: Map tab of AS paths.
* [Google Geolocation API](https://developers.google.com/maps/documentation/geolocation/): Locating the user's address on the map.
* [Pretty Checkboxes](https://lokesh-coder.github.io/pretty-checkbox/): GUI slider switches.
* [JQuery](https://jquery.com/): Code style JavaScript querying of DOM elements.
* [JQuery Knob](http://anthonyterrien.com/demo/knob/): Stylized dials for the bandwidth controls.
* [TopoJSON](https://github.com/topojson/topojson): Geospatial utilities *(possibly deprecated)*

# SCIONLab VM Test Development

Add alternate test forwarding port line in `Vagrantfile`:
```
  config.vm.network "forwarded_port", guest: 8080, host: 8080, protocol: "tcp"
```

Update Go Paths:
```shell
echo 'export GOPATH="$HOME/go"' >> ~/.profile
echo 'export PATH="$HOME/.local/bin:$GOPATH/bin:/usr/local/go/bin:$PATH"' >> ~/.profile
source ~/.profile
mkdir -p "$GOPATH"
```

Install Go 1.14:
```shell
cd ~
sudo rm -rf /usr/local/go
curl -LO https://golang.org/dl/go1.14.12.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.14.12.linux-amd64.tar.gz
```

Build and install `scion-apps`:
```shell
sudo apt install make gcc libpam0g-dev
cd ~
git clone -b scionlab https://github.com/netsec-ethz/scion-apps
cd scion-apps
make install
```

Download scionlab's fork of scion and build and install `sig`:
```shell
cd ~
git clone -b scionlab https://github.com/netsec-ethz/scion
go build -o $GOPATH/bin/sig ~/scion/go/sig/main.go
```

Install Go Watcher:
```shell
go get -u github.com/mitranim/gow 
```

Development Run:
```shell
cd ~/scion-apps/webapp
gow run . \
-a 0.0.0.0 \
-p 8080 \
-r ./web/data \
-srvroot ./web \
-sabin /usr/bin/scion \
-sroot /etc/scion \
-sbin /usr/bin \
-sgen  /etc/scion/gen \
-sgenc /var/lib/scion \
-slogs /var/log/scion
```
