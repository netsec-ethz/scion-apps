.PHONY: all clean install

ROOT_DIR=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
SRCDIRS= helloworld sensorapp/sensorserver sensorapp/sensorfetcher camerapp/imageserver camerapp/imagefetcher bwtester/bwtestserver bwtester/bwtestclient bat bat/example_server tools/pathdb_dump ssh/client ssh/server netcat webapp
TARGETS = $(foreach D,$(SRCDIRS),$(D)/$(notdir $(D)))

all: $(TARGETS)

clean:
	go clean ./...

install: all
	@$(foreach d,$(SRCDIRS), cd $(ROOT_DIR)/$(d); cp $(shell basename $(d)) ~/go/bin;)

# using eval to create as many rules as we have $TARGETS
# each target corresponds to the binary file name (e.g. sensorapp/sensorserver/sensorserver)
define gobuild_tmpl =
.PHONY: $(1)
$(1): go.mod $(2)
	go build -o $$(dir $$@) ./$$(dir $$@)
endef
$(foreach D,$(TARGETS),$(eval $(call gobuild_tmpl, $(D), $(shell find $(dir $(D)) -name '*.go') )))
