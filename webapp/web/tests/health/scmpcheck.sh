#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# allow IA via args, ignoring gen/ia
ia=$(echo $1 | sed "s/_/:/g")
iaFile=$(echo $1 | sed "s/:/_/g")
echo "IA found: $iaFile"

# get local IP
ip=$(hostname -I | cut -d" " -f1)
echo "IP found: $ip"

isd=$(echo ${iaFile} | cut -d"-" -f1)
as=$(echo ${iaFile} | cut -d"-" -f2)
topologyFile=$SCION_GEN/ISD$isd/AS$as/endhost/topology.json

# get remote addresses from interfaces, return paired list
dsts=($(cat $topologyFile | python3 -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
for b in brs:
    for i in brs[b]['Interfaces']:
        print(brs[b]['Interfaces'][i]['ISD_AS'])
        print(brs[b]['Interfaces'][i]['RemoteOverlay']['Addr'])"))

# test scmp echo on each interface
for ((i=0; i<${#dsts[@]}; i+=2))
do
    # if no response under default scmp ping timeout consider connection failed
    ia_dst="${dsts[i]}"
    ip_dst="${dsts[i+1]}"
    cmd="$SCION_BIN/scmp echo -c 1 -timeout 5s -local $ia,[$ip] -remote $ia_dst,[$ip_dst]"
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
done

exit $?
