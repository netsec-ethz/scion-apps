# Webapp Development Notes

Your mileage may vary.

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

Install Go 1.13:
```shell
cd ~
curl -O https://dl.google.com/go/go1.13.10.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.13.10.linux-amd64.tar.gz
```

Build and install `scion-apps`:
```shell
sudo apt install make gcc libpam0g-dev
cd ~
git clone -b scionlab https://github.com/netsec-ethz/scion-apps
cd scion-apps
mkdir ~/go/bin # needed for make bug
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

Useful URLs Firefox:
- <about:webrtc>

Useful URLs Chrome:
- <chrome://webrtc-internals>
