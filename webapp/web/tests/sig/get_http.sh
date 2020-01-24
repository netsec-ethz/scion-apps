#!/bin/bash

mkdir ${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1
file=${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.sh
cat >$file <<EOL
#!/bin/bash

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

# Add some client on host B:
sudo ip link add client type dummy
sudo ip addr add 172.16.${IdLocal}.1/24 brd + dev client label client:0

# Query the server running on host A from host B:
curl --interface 172.16.${IdLocal}.1 172.16.${IdRemote}.1:${ServePort}/sighello.html

# You should see the “Hello World!” message as output from the last command.
EOL
cat $file
