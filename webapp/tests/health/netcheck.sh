#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
iaFile=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/:/_/g")
echo "IA found: $iaFile"

isd=$(echo ${iaFile} | cut -d"-" -f1)
as=$(echo ${iaFile} | cut -d"-" -f2)
topologyFile=$SC/gen/ISD$isd/AS$as/endhost/topology.json

ip_dst=$(cat $topologyFile | python -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
interfaces=next(iter(brs.values()))['Interfaces']
inter=next(iter(interfaces.values()))
print inter['RemoteOverlay']['Addr']")
echo $ip_dst

cmd="ping -c 1 -w 5 $ip_dst"
echo "Running: $cmd"
recv=$($cmd | grep -E -o '[0-9]+ received' | cut -f1 -d' ')
if [ "$recv" != "1" ]; then
    echo "ICMP ping failed from $ip_dst."
    exit 1
else
    echo "ICMP ping succeeded."
fi
exit $?
