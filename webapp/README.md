Webapp AS Visualization
=========================

More installation and usage information is available on the [SCION Tutorials web page for webapp](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp/).

## Webapp
Webapp is a Go application that will serve up a static web portal to make it easy to visualize and experiment with SCIONLab test apps on a virtual machine.


## Packaged Setup/Run
For running `webapp` in a package environment, like the default [SCIONLab](https://www.scionlab.org) environment, it will require command-line options for `webapp` to find the tools its requires. In most cases, the default `scionlab` and `scion-apps` packages will need to be specified on the command-line.

To run the Go Web UI at a specific address `-a` and port `-p` like 0.0.0.0:8000 for a SCIONLab VM use:
```shell
webapp \
-a 0.0.0.0 \
-p 8000 \
-r /var/lib/scion/webapp/web/data \
-srvroot /var/lib/scion/webapp/web \
-sabin /usr/bin \
-sroot /etc/scion \
-sbin /usr/bin \
-sgen  /etc/scion/gen \
-sgenc /var/lib/scion \
-slogs /var/log/scion
```
Now, open a web browser at [http://127.0.0.1:8000](http://127.0.0.1:8000), to begin.


## Development Setup/Run
For running `webapp` in a development environment for the SCION Infrastructure, follow the SCIONLab development install and run process at [https://github.com/netsec-ethz/netsec-scion](https://github.com/netsec-ethz/netsec-scion).

Then, follow these steps to install SCIONLab Apps to run `webapp` in development.

### Development Install
```shell
mkdir ~/go/src/github.com/netsec-ethz
cd ~/go/src/github.com/netsec-ethz
git clone https://github.com/netsec-ethz/scion-apps.git
```

### Development Build
Install all [SCIONLab apps](https://github.com/netsec-ethz/scion-apps) and dependencies, including `webapp`:
```shell
cd scion-apps
./deps.sh
make install
```

### Development Run
You can alter the defaults on the command line, all of which are listed below:
```shell
webapp \
-a 127.0.0.1 \
-p 8000 \
-r . \
-srvroot $GOPATH/src/github.com/netsec-ethz/scion-apps/webapp/web \
-sabin $GOPATH/bin \
-sroot $GOPATH/src/github.com/scionproto/scion \
-sbin $GOPATH/src/github.com/scionproto/scion/bin \
-sgen $GOPATH/src/github.com/scionproto/scion/gen \
-sgenc $GOPATH/src/github.com/scionproto/scion/gen-cache \
-slogs $GOPATH/src/github.com/scionproto/scion/logs
```
or can you run `webapp` like this, which will use the defaults above:
```shell
webapp
```

## Dependancies
A list of dependencies for `webapp` can be found at [dependencies.md](./dependencies.md).

## Help
```shell
Usage of webapp:
  -a string
        Address of server host. (default "127.0.0.1")
  -p int
        Port of server host. (default 8000)
  -r string
        Root path to read/browse from, CAUTION: read-access granted from -a and -p. (default ".")
  -sabin string
        Path to execute the installed scionlab apps binaries (default "/home/ubuntu/go/bin")
  -sbin string
        Path to execute SCION bin directory of infrastructure tools (default "/home/ubuntu/go/src/github.com/scionproto/scion/bin")
  -sgen string
        Path to read SCION gen directory of infrastructure config (default "/home/ubuntu/go/src/github.com/scionproto/scion/gen")
  -sgenc string
        Path to read SCION gen-cache directory of infrastructure run-time config (default "/home/ubuntu/go/src/github.com/scionproto/scion/gen-cache")
  -slogs string
        Path to read SCION logs directory of infrastructure logging (default "/home/ubuntu/go/src/github.com/scionproto/scion/logs")
  -sroot string
        Path to read SCION root directory of infrastructure (default "/home/ubuntu/go/src/github.com/scionproto/scion")
  -srvroot string
        Path to read/write web server files. (default "/home/ubuntu/go/src/github.com/netsec-ethz/scion-apps/webapp/web")
```

## Related Links
* [Webapp SCIONLab AS Visualization Tutorials](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp/)
* [Webapp SCIONLab Apps Visualization](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp_apps/)
* [Webapp Development Tips](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp_development/)
