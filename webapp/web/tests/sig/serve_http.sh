#!/bin/bash
exit

sudo modprobe dummy


sudo ip link set name dummy11 dev dummy0
sudo ip addr add 172.16.0.11/32 brd + dev dummy11 label dummy11:0


sudo ip rule add to 172.16.12.0/24 lookup 11 prio 11

$SC/bin/sig -config=/home/ubuntu/go/src/github.com/scionproto/scion/gen/ISD${ISD}/AS${AS}/sig${IA}-1/sigA.config > $SC/logs/sig${IA}-1.log 2>&1 &

sudo ip rule show

sudo ip link add server type dummy
sudo ip addr add 172.16.12.1/24 brd + dev server label server:0

mkdir $SC/WWW
echo "Hello World!" > $SC/WWW/hello.html
cd $SC/WWW/ && python3 -m http.server --bind 172.16.12.1 8081 &
