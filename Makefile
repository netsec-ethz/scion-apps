.PHONY: all clean

ROOT_DIR=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
SRCDIRS= helloworld sensorapp/sensorserver sensorapp/sensorfetcher camerapp/imageserver camerapp/imagefetcher bwtester/bwtestserver bwtester/bwtestclient webapp bat bat/example_server tools/pathdb_dump ssh/client ssh/server netcat
TARGETS = $(foreach D,$(SRCDIRS),$(D)/$(notdir $(D)))

all: $(TARGETS)

deps:
	./deps.sh

vendor: deps

clean:
	@$(foreach d,$(SRCDIRS),cd $(ROOT_DIR)/$(d) && go clean;)

mrproper: clean
	rm -rf vendor/*/

install: all
	@$(foreach d,$(SRCDIRS), cd $(ROOT_DIR)/$(d); cp $(shell basename $(d)) ~/go/bin;)

# using eval to create as many rules as we have $TARGETS
# each target corresponds to the binary file name (e.g. sensorapp/sensorserver/sensorserver)
define gobuild_tmpl = 
$(1): $(2)
	cd $$(dir $$@) && go build
endef
$(foreach D,$(TARGETS),$(eval $(call gobuild_tmpl, $(D), $(shell find $(dir $(D)) -name '*.go') )))
