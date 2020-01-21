#!/bin/bash
exit

sudo modprobe dummy


sudo ip link add dummy12 type dummy
sudo ip addr add 172.16.0.12/32 brd + dev dummy12 label dummy12:0


sudo ip rule add to 172.16.11.0/24 lookup 12 prio 12

$SC/bin/sig -config=/home/ubuntu/go/src/github.com/scionproto/scion/gen/ISD${ISD}/AS${AS}/sig${IA}-1/sigB.config > $SC/logs/sig${IA}-1.log 2>&1 &

sudo ip rule show

sudo ip link add client type dummy
sudo ip addr add 172.16.11.1/24 brd + dev client label client:0


curl --interface 172.16.11.1 172.16.12.1:8081/hello.html
