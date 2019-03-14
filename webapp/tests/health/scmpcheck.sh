#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
iaFile=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/:/_/g")
ia=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/_/:/g")
echo "IA found: $iaFile"

# get local IP
ip=$(hostname -I | cut -d" " -f1)
echo "IP found: $ip"

isd=$(echo ${iaFile} | cut -d"-" -f1)
as=$(echo ${iaFile} | cut -d"-" -f2)
topologyFile=$SC/gen/ISD$isd/AS$as/endhost/topology.json

ip_dst=$(cat $topologyFile | python -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
interfaces=next(iter(brs.values()))['Interfaces']
inter=next(iter(interfaces.values()))
print inter['RemoteOverlay']['Addr']")
ia_dst=$(cat $topologyFile | python -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
interfaces=next(iter(brs.values()))['Interfaces']
inter=next(iter(interfaces.values()))
print inter['ISD_AS']")

# if no response under default scmp ping timeout consider connection failed
cmd="$SC/bin/scmp echo -c 1 -timeout 5s -local $ia,[$ip] -remote $ia_dst,[$ip_dst]"
if [ $isd -lt 16 ]; then
    # local tests
    cmd="$cmd -sciondFromIA"
fi
echo "Running: $cmd"
recv=$($cmd | grep -E -o '[0-9]+ received' | cut -f1 -d' ')
if [ "$recv" != "1" ]; then
    echo "SCMP echo failed from $ia_dst,[$ip_dst]."
    exit 1
else
    echo "SCMP echo succeeded."
fi
exit $?
