#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
ia_file=`~/go/src/github.com/scionproto/scion/gen/ia`
ia=`cat $ia_file`

# format log file and beacons grep string
fsafe_ia=$(echo $ia | sed "s/:/_/g")
logfile=`~/go/src/github.com/scionproto/scion/logs/bs${fsafe_ia}-1.info`
grep_str="Cert chain request received for $ia"

echo "IA is '$ia'"
if grep -c "$grep_str" "$logfile"; then
    echo "No PCBs received yet."
    exit 1
fi

exit $?
