#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# check for required binaries which may not have been built and installed
missingbin=false
declare -a apps=("$SCION_BIN/scmp" "$SCION_BIN/sig" "$APPS_ROOT/bwtestclient" "$APPS_ROOT/imagefetcher" "$APPS_ROOT/sensorfetcher" )
for fbin in "${apps[@]}"; do
    if [ -f "$fbin" ]; then
        echo "$fbin exists"
    else
        echo "$fbin does not exist, 'make install' may be needed" 1>&2
        missingbin=true
    fi
done

if [ "$missingbin" = true ] ; then
    exit 1
fi
