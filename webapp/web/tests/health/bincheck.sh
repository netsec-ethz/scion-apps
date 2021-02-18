#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# check for required binaries which may not have been built and installed
missingbin=false
declare -a apps=("scion" "scion-bwtestclient" "scion-imagefetcher" "scion-sensorfetcher" )
for a in "${apps[@]}"; do
    path=$(which $a)
    if [ -x "$path" ]; then
        echo "$a exists"
    else
        echo "$a does not exist, check that SCION and SCION Apps are installed and that PATH is set correctly." 1>&2
        missingbin=true
    fi
done

if [ "$missingbin" = true ] ; then
    exit 1
fi
