#!/bin/bash
exit

#create two dummy interfaces:
sudo modprobe dummy

sudo ip link add dummy${IdRemote} type dummy
sudo ip addr add 172.16.0.${IdRemote}/32 brd + dev dummy${IdRemote} label dummy${IdRemote}:0

# Now we need to add the routing rules for the two SIGs:
sudo ip rule add to 172.16.${IdLocal}.0/24 lookup ${IdRemote} prio ${IdRemote}

# Now start the two SIGs with the following commands:
${SCION_BIN}/sig -config=${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.config > ${SCION_LOGS}/sig${IA}-1.log 2>&1 &

# To show the ip rules and routes
sudo ip rule show
sudo ip route show table ${IdLocal}
sudo ip route show table ${IdRemote}

# Add some server on host A:
sudo ip link add server type dummy
sudo ip addr add 172.16.${IdLocal}.1/24 brd + dev server label server:0

now=$(date)
SIG_SERVE=${STATIC_ROOT}/data/www/sig${IA}
mkdir ${SIG_SERVE}
echo "Hello World from ${IAd} at ${now}!" > ${SIG_SERVE}/sighello.html
cd ${SIG_SERVE} && python3 -m http.server --bind 172.16.${IdLocal}.1 ${ServePort} &
