#!/bin/bash
i=0
while [ $i -lt 3 ]; do
    # Query the cs metrics server
    response=$(curl --silent $4)
    if echo "$response" | grep -q "beacondb_results_total.*result\=\"ok_success"; then
        exit 0
    fi
    i=$((i+1))
    sleep 0.5
done
exit 1
