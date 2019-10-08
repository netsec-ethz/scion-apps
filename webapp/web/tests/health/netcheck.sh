#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# allow IA via args, ignoring gen/ia
iaFile=$(echo $1 | sed "s/:/_/g")
echo "IA found: $iaFile"

isd=$(echo ${iaFile} | cut -d"-" -f1)
as=$(echo ${iaFile} | cut -d"-" -f2)
topologyFile=$SCION_GEN/ISD$isd/AS$as/endhost/topology.json

# get remote addresses from interfaces
ip_dsts=$(cat $topologyFile | python3 -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
for b in brs:
    for i in brs[b]['Interfaces']:
        print(brs[b]['Interfaces'][i]['RemoteOverlay']['Addr'])")

# test icmp ping on each interface
for ip_dst in $ip_dsts
do
    cmd="ping -c 1 -w 5 $ip_dst"
    echo "Running: $cmd"
    recv=$($cmd | grep -E -o '[0-9]+ received' | cut -f1 -d' ')
    if [ "$recv" != "1" ]; then
        echo "ICMP ping failed from $ip_dst."
        exit 1
    else
        echo "ICMP ping succeeded."
    fi
done

exit $?
