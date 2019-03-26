#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
iaFile=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/:/_/g")
ia=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/_/:/g")
echo "IA found: $ia"

# get local IP
ip=$(hostname -I | cut -d" " -f1)
echo "IP found: $ip"

isd=$(echo ${iaFile} | cut -d"-" -f1)
as=$(echo ${iaFile} | cut -d"-" -f2)
topologyFile=$SC/gen/ISD$isd/AS$as/endhost/topology.json

# get remote addresses from interfaces
ip_dsts=$(cat $topologyFile | python -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
for b in brs:
    for i in brs[b]['Interfaces']:
        print brs[b]['Interfaces'][i]['RemoteOverlay']['Addr']")
ia_dsts=($(cat $topologyFile | python -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
for b in brs:
    for i in brs[b]['Interfaces']:
        print brs[b]['Interfaces'][i]['ISD_AS']"))

# test scmp echo on each interface
i=0
for ip_dst in $ip_dsts
do
    # if no response under default scmp ping timeout consider connection failed
    ia_dst="${ia_dsts[i]}"
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
    ((i = i + 1))
done

exit $?
