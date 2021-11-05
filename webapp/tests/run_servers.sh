#!/bin/bash
# Build and run test servers to emulate test endpoints on localhost for webapp
dstIA=1-ff00:0:112
dstIP=127.0.0.2

# test bwtest server
echo "Running test bwtest server..."
bwtestserver -s ${dstIA},[${dstIP}]:30100 -sciondFromIA &

# test sensor server
echo "Running test sensor server..."
python3 ${GOPATH}/src/github.com/netsec-ethz/scion-apps/sensorapp/sensorserver/timereader.py | sensorserver -s ${dstIA},[${dstIP}]:42003 -sciondFromIA &

# test scmp echo
# dispatcher is responsible for responding to echo

# test scmp traceroute
# dispatcher is responsible for responding to traceroute

# test pingpong server
cd $SC
echo "Running test pingpongserver..."
./bin/pingpong -mode server -local ${dstIA},[${dstIP}]:40002 -sciondFromIA &
