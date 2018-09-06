SCIONLab Go Static Tester
=========================

## Webapp Setup

Webapp is a Go application that will serve up a static web portal to make it easy to
experiment with SCIONLab test apps on a virtual machine.

To install the SCIONLab `webapp`:

```shell
go get github.com/perrig/scionlab/webapp
```

### Local Infrastructure

To run the Go Web UI at default localhost 127.0.0.1:8000 run:

```shell
webapp
```

### SCIONLab Virtual Machine

Using vagrant, make sure to edit your `vagrantfile` to provision the additional port
for the Go web server by adding this line for port 8080 (for example, just choose any forwarding
port not already in use by vagrant):

```
config.vm.network "forwarded_port", guest: 8080, host: 8080, protocol: "tcp"
```

To run the Go Web UI at a specific address (-a) and port (-p) like 0.0.0.0:8080 for a SCIONLabVM use:

```shell
webapp -a 0.0.0.0 -p 8080 -r .
```

Now, open a web browser at http://127.0.0.1:8080, to begin.

## Development

For developing the web server go code, since it is annoying to make several changes,
only to have to start and stop the web server each time, a watcher library
like `go-watcher` is recommended.

```shell
go get github.com/canthefason/go-watcher
go install github.com/canthefason/go-watcher/cmd/watcher
```

After installation you can `cd` to the `webapp.go` directory and webapp will be rebuilt
and rerun every time you save your source file changes, with or without command arguments.

```shell
cd ~/go/src/github.com/perrig/scionlab/webapp
watcher -a 0.0.0.0 -p 8080 -r ..
```
or
```shell
cd ~/go/src/github.com/perrig/scionlab/webapp
watcher
```

## Webapp Features

This Go web server wraps several SCION test client apps and provides an interface
for any text and/or image output received.
[SCIONLab Apps](http://github.com/perrig/scionlab) are on Github.

Two functional server tests are included to test the networks without needing
specific sensor or camera hardware, `imagetest` and `statstest`.

Supported client applications include `camerapp`, `sensorapp`, and `bwtester`.
For best results, ensure the desired server-side apps are running and connected to
the SCION network first. Instructions to setup the servers are
[here](https://github.com/perrig/SCIONLab/blob/master/README.md).
The web interface launched above can be used to run the client-side apps.

### File System Browser

The File System Browser button on the front page will allow you to navigate and serve any
files on the SCIONLab node from the root (-r) directory you specified (if any) when
starting webapp.go.

### bwtester client

Simply adjust the dials to the desired level, while the icon lock will allow you
to keep one value constant.

![Webapp Bandwidth Test](static/img/bwtest.png?raw=true "Webapp Bandwidth Test")


### camerapp client

The retrieved image will appear scaled down and can be clicked on to open a larger size.

![Webapp Image Test](static/img/imagetest.png?raw=true "Webapp Image Test")


### sensorapp client

![Webapp Stats Test](static/img/statstest.png?raw=true "Webapp Stats Test")


### statstest server

This hardware-independent test will echo some remote machine stats from the Python script
`local-stats.py`, which is piped to the server for transmission to clients.
On your remote SCION server node run (substituting your own address parameters):

```shell
cd $GOPATH/src/github.com/perrig/scionlab/webapp/tests/statstest/statsserver
python3 local-stats.py | sensorserver -s 1-15,[127.0.0.5]:35555
```

Now, from your webapp browser interface running on your virtual client SCION node,
you can enter both client and server addresses and ask the client for remote stats
by clicking on the `sensorapp` tab.

### imagetest server

This hardware-independent test will generate an image with some remote machine stats from
the Go app `local-image.go`, which will be saved locally for transmission to clients.

You may need golang.org's image package first:

```shell
go get golang.org/x/image
```

On your remote SCION server node run (substituting your own address parameters):

```shell
cd $GOPATH/src/github.com/perrig/scionlab/webapp/tests/imgtest/imgserver
go build
./imgserver | imageserver -s 1-18,[127.0.0.8]:38887
```

Now, from your webapp browser interface running on your virtual client SCION node,
you can enter both client and server addresses and ask the client for the most
recently generated remote image by clicking on the `camerapp` tab.

