.PHONY: all clean test lint build install

BIN = bin
# Default DESTDIR for installation uses fallback sequence, as documented by go install;
#   This Make-escaped ($ replaced with $$) shell oneliner sources the
#   environment as returned by go env, and uses the "Default Values" parameter
#   expansion ${variable:-default} to implement the fallback sequence:
#     $GOBIN, else $GOPATH/bin, else $HOME/go/bin
DESTDIR = $(shell set -a; eval $$( go env ); echo $${GOBIN:-$${GOPATH:-$${HOME}/go}/bin})

# HINT: build with TAGS=norains to build without rains support
TAGS =

all: lint build

build: scion-bat \
	scion-bwtestclient scion-bwtestserver \
	scion-imagefetcher scion-imageserver \
	scion-netcat \
	scion-sensorfetcher scion-sensorserver \
	scion-ssh scion-sshd \
	scion-ftp scion-ftpserver \
	example-helloworld \
	example-shttp-client example-shttp-server example-shttp-fileserver example-shttp-proxy

clean:
	go clean ./...
	rm -f bin/*

test: lint
	go test -v -tags=$(TAGS) ./...

setup_lint:
	@# Install golangci-lint (as dumb as this looks, this is the recommended way to install)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $$(go env GOPATH)/bin v1.26.0

lint:
	@type golangci-lint > /dev/null || ( echo "golangci-lint not found. Install it manually or by running 'make setup_lint'."; exit 1 )
	golangci-lint run --build-tags=$(TAGS)

install: all
  # Note: install everything but the examples
	mkdir -p $(DESTDIR)
	cp -t $(DESTDIR) $(BIN)/scion-*

integration: all
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

.PHONY: scion-ssh
scion-ssh:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ssh/client/

.PHONY: scion-sshd
scion-sshd:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ssh/server/

.PHONY: scion-webapp
scion-webapp:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./webapp/

.PHONY: scion-ftp
scion-ftp:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ftp/client/scionftp/

.PHONY: scion-ftpserver
scion-ftpserver:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./ftp/server/scionftp_server/

.PHONY: example-helloworld
example-helloworld:
	go build -tags=$(TAGS) -o $(BIN)/$@ ./_examples/helloworld/

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
