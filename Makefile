.PHONY: all clean test lint build install

BIN = bin
# default DESTDIR for installation uses fallback sequence, as documented by go install.
DESTDIR = $(shell go env GOBIN GOPATH | eval; echo $${GOBIN:-$${GOPATH:-$${HOME}/go}/bin})

all: lint build

build: scion-bat \
	scion-bwtestclient scion-bwtestserver \
	scion-imagefetcher scion-imageserver \
	scion-netcat \
	scion-sensorfetcher scion-sensorserver \
	scion-ssh scion-sshd \
	example-helloworld \
	example-shttp-client example-shttp-server

clean:
	go clean ./...
	rm -f bin/*

test: lint
	go test -v ./...

setup_lint:
	@# Install golangci-lint (as dumb as this looks, this is the recommended way to install)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $$(go env GOPATH)/bin v1.26.0

lint:
	@type golangci-lint > /dev/null || ( echo "golangci-lint not found. Install it manually or by running 'make setup_lint'."; exit 1 )
	golangci-lint run

install: all
  # Note: install everything but the examples
	mkdir -p $(DESTDIR)
	cp -t $(DESTDIR) $(BIN)/scion-*

integration: all
	go test -v -tags=integration ./... ./_examples/helloworld/

.PHONY: scion-bat
scion-bat:
	go build -o $(BIN)/$@ ./bat/

.PHONY: scion-bwtestclient
scion-bwtestclient:
	go build -o $(BIN)/$@ ./bwtester/bwtestclient/

.PHONY: scion-bwtestserver
scion-bwtestserver:
	go build -o $(BIN)/$@ ./bwtester/bwtestserver/

.PHONY: scion-imagefetcher
scion-imagefetcher:
	go build -o $(BIN)/$@ ./camerapp/imagefetcher/

.PHONY: scion-imageserver
scion-imageserver:
	go build -o $(BIN)/$@ ./camerapp/imageserver/

.PHONY: scion-netcat
scion-netcat:
	go build -o $(BIN)/$@ ./netcat/

.PHONY: scion-sensorfetcher
scion-sensorfetcher:
	go build -o $(BIN)/$@ ./sensorapp/sensorfetcher/

.PHONY: scion-sensorserver
scion-sensorserver:
	go build -o $(BIN)/$@ ./sensorapp/sensorserver/

.PHONY: scion-ssh
scion-ssh:
	go build -o $(BIN)/$@ ./ssh/client/

.PHONY: scion-sshd
scion-sshd:
	go build -o $(BIN)/$@ ./ssh/server/

.PHONY: scion-webapp
scion-webapp:
	go build -o $(BIN)/$@ ./webapp/

.PHONY: example-helloworld
example-helloworld:
	go build -o $(BIN)/$@ ./_examples/helloworld/

.PHONY: example-shttp-client
example-shttp-client:
	go build -o $(BIN)/$@ ./_examples/shttp/client

.PHONY: example-shttp-server
example-shttp-server:
	go build -o $(BIN)/$@ ./_examples/shttp/server
