.PHONY: all clean test lint build install

BIN = bin
# Default DESTDIR for installation uses fallback sequence, as documented by go install;
#   This Make-escaped ($ replaced with $$) shell oneliner sources the
#   environment as returned by go env, and uses the "Default Values" parameter
#   expansion ${variable:-default} to implement the fallback sequence:
#     $GOBIN, else $GOPATH/bin, else $HOME/go/bin
#   If there are multiple entries in GOPATH, take the first.
DESTDIR = $(shell set -a; eval $$( go env ); gopath=$${GOPATH%:*}; echo $${GOBIN:-$${gopath:-$${HOME}/go}/bin})

# HINT: build with TAGS=norains to build without rains support
TAGS =

all: build lint

build: scion-bat \
	scion-bwtestclient scion-bwtestserver \
	scion-imagefetcher scion-imageserver \
	scion-netcat \
	scion-sensorfetcher scion-sensorserver \
	scion-skip \
	scion-ssh scion-sshd \
	scion-webapp \
	example-helloworld \
	example-helloquic \
	example-hellodrkey \
	example-shttp-client example-shttp-server example-shttp-fileserver example-shttp-proxy

clean:
	go clean ./...
	rm -f bin/*

test:
	go test -v -tags=$(TAGS) ./...

setup_lint:
	@# Install golangci-lint (as dumb as this looks, this is the recommended way to install)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ${DESTDIR} v1.37.1

lint:
	@type golangci-lint > /dev/null || ( echo "golangci-lint not found. Install it manually or by running 'make setup_lint'."; exit 1 )
	golangci-lint run --build-tags=$(TAGS) --timeout=2m0s -v

install: all
  # Note: install everything but the examples
	mkdir -p $(DESTDIR)
	cp -t $(DESTDIR) $(BIN)/scion-*

integration: build
	go test -v -tags=integration,$(TAGS) ./... ./_examples/helloworld/

.PHONY: scion-bat
scion-bat:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./bat/

.PHONY: scion-bwtestclient
scion-bwtestclient:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./bwtester/bwtestclient/

.PHONY: scion-bwtestserver
scion-bwtestserver:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./bwtester/bwtestserver/

.PHONY: scion-imagefetcher
scion-imagefetcher:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./camerapp/imagefetcher/

.PHONY: scion-imageserver
scion-imageserver:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./camerapp/imageserver/

.PHONY: scion-netcat
scion-netcat:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./netcat/

.PHONY: scion-sensorfetcher
scion-sensorfetcher:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./sensorapp/sensorfetcher/

.PHONY: scion-sensorserver
scion-sensorserver:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./sensorapp/sensorserver/

.PHONY: scion-skip
scion-skip:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./skip/

.PHONY: scion-ssh
scion-ssh:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ssh/client/

.PHONY: scion-sshd
scion-sshd:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ssh/server/

.PHONY: scion-webapp
scion-webapp:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./webapp/

.PHONY: example-helloworld
example-helloworld:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/helloworld/

.PHONY: example-helloquic
example-helloquic:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/helloquic/

.PHONY: example-shttp-client
example-shttp-client:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/shttp/client

.PHONY: example-shttp-server
example-shttp-server:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/shttp/server

.PHONY: example-shttp-fileserver
example-shttp-fileserver:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/shttp/fileserver

.PHONY: example-shttp-proxy
example-shttp-proxy:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/shttp/proxy

.PHONY: example-hellodrkey
example-hellodrkey:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/hellodrkey/
