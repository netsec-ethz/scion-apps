Webapp AS Visualization
=========================

More installation and usage information is available on the [SCION Tutorials web page for webapp](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp/).

## Webapp Setup
Webapp is a Go application that will serve up a static web portal to make it easy to visualize and experiment with SCIONLab test apps on a virtual machine.

### Install
```shell
mkdir ~/go/src/github.com/netsec-ethz
cd ~/go/src/github.com/netsec-ethz
git clone https://github.com/netsec-ethz/scion-apps.git
```

### Build
Install all [SCIONLab apps](https://github.com/netsec-ethz/scion-apps) and dependancies, including `webapp`:
```shell
cd scion-apps
./deps.sh
make install
```

### Run
!!! warning
    If the old [scion-viz](https://github.com/netsec-ethz/scion-viz) web server is running on your SCIONLab VM, port 8000 may still be in use. To remedy this, before `vagrant up`, make sure to edit your `vagrantfile` to provision an alternate port for the `webapp` web server. Add this line for a different port, say 8080 (for example, just choose any forwarding port not already in use by vagrant, and use that port everywhere below):

    ```
    config.vm.network "forwarded_port", guest: 8080, host: 8080, protocol: "tcp"
    ```

To run the Go Web UI at a specific address (-a) and port (-p) like 0.0.0.0:8000 for a SCIONLab VM use:
```shell
cd webapp
webapp -a 0.0.0.0 -p 8000
```
Now, open a web browser at [http://127.0.0.1:8000](http://127.0.0.1:8000), to begin.

## Related Links
* [Webapp SCIONLab AS Visualization Tutorials](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp/)
* [Webapp SCIONLab Apps Visualization](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp_apps/)
* [Webapp Development Tips](https://netsec-ethz.github.io/scion-tutorials/as_visualization/webapp_development/)
