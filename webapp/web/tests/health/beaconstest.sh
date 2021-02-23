#!/bin/bash
i=0
while [ $i -lt 3 ]; do
    # Query the cs metrics server
    response=$(curl --silent $4)
    if echo "$response" | grep "control_beaconing_registered_segments_total" | grep -q 'result="ok'; then
        exit 0
    fi
    i=$((i+1))
    sleep 0.5
done
exit 1
