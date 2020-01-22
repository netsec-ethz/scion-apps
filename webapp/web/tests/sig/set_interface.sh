#!/bin/bash
exit

# Create the configuration directories for the SIGs,
mkdir -p ${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/

# build the SIG binary
#go build -o ${SCION_BIN}/sig ${SC}/go/sig/main.go

# set the linux capabilities on the binary:
sudo setcap cap_net_admin+eip ${SCION_BIN}/sig

# Enable routing:
sudo sysctl net.ipv4.conf.default.rp_filter=0
sudo sysctl net.ipv4.conf.all.rp_filter=0
sudo sysctl net.ipv4.ip_forward=1
