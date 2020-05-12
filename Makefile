.PHONY: all clean test lint build install

ROOT_DIR=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
SRCDIRS= sensorapp/sensorserver sensorapp/sensorfetcher camerapp/imageserver camerapp/imagefetcher bwtester/bwtestserver bwtester/bwtestclient bat ssh/client ssh/server netcat webapp _examples/helloworld _examples/shttp/client _examples/shttp/server
BUILD_TARGETS = $(foreach D,$(SRCDIRS),$(D)/$(notdir $(D)))

all: lint build

clean:
	go clean ./...

test: lint
	go test -v ./...

setup_lint:
	@# Install golangci-lint (as dumb as this looks, this is the recommended way to install)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $$(go env GOPATH)/bin v1.23.1

lint:
	@type golangci-lint > /dev/null || ( echo "golangci-lint not found. Install it manually or by running 'make setup_lint'."; exit 1 )
	golangci-lint run

build: $(BUILD_TARGETS)

install: all
	@$(foreach d,$(SRCDIRS), cd $(ROOT_DIR)/$(d); cp $(shell basename $(d)) /go/bin;)

integration: install
	go test -v -tags=integration ./...

# using eval to create as many rules as we have $BUILD_TARGETS
# each target corresponds to the binary file name (e.g. sensorapp/sensorserver/sensorserver)
define gobuild_tmpl =
.PHONY: $(1)
$(1): go.mod $(2)
	go build -o $$(dir $$@) ./$$(dir $$@)
endef
$(foreach D,$(BUILD_TARGETS),$(eval $(call gobuild_tmpl, $(D), $(shell find $(dir $(D)) -name '*.go') )))
