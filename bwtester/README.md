# bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network. This document describes the design of the code and protocol. Instructions on the installation and usage are described in the main [README.md](https://github.com/netsec-ethz/scion-apps/blob/master/README.md).

## Protocol design

The goal is to set up bandwidth test servers throughout the SCION network, which enable stress testing of the data plane infrastructure.

To avoid server bottlenecks biasing the results, a server only allows a single client to perform a bandwidth test at a given point in time. Clients are served on a first-come-first-served basis. We limit the duration of each test to 10 seconds.

A bandwidth test is parametrized by the following parameters, which is specified separately for the client->server and server->client direction:

```go
type BwtestParameters struct {
	BwtestDuration time.Duration
	PacketSize     int
	NumPackets     int
	PrgKey         []byte
	Port           uint16
}
```

The duration can be up to 10 seconds, the packet size needs to be at least 4 bytes. The duration, packet size, and number of packets determine the bandwidth, as NumPackets of size PacketSize are sent during BwtestDuration.

The packet contents are filled with a Pseudo-Random Generator (PRG) based on AES, the 128-bit long key is encoded in the 16-byte long slice PrgKey. The port number determines the sending port, the receiving port is specified in the other parameter list.

## Wireline data format

The wireline protocol is as follows:
* 'N' new bwtest request
  > Request: 'N', encoded bwtest parameters client->server, encoded bwtest parameters server->client
  > 
  > Success response: 'N', 0
  > 
  > Failure response: 'N', number of seconds to wait until next request is sent
* 'R' result request
  > Request: 'R', encoded client sending PRG key
  >
  > Success response: 'R', 0, encoded result data
  >
  > Not ready response: 'R', number of seconds to wait until result should be ready by
  >
  > Not found response: 'R', 127

## bwtestclient

The client application reads the command line parameters and establishes two SCION UDP connections to the bwtestserver: a Control Connection (CC) and a Data Connection (DC). The port numbers for the DC are simply picked as one larger than the respective ports of the CC (the CC port numbers are passed on the command line).

To achieve reliability for the initial request, it may be retried up to 5 times. If the server responds with a number of seconds to wait, that amount of time is waited off before another request is sent (as the server only serves a single client at a time). Reliability for fetching the results is achieved in the same way.

## bwtestserver

The server runs a main loop that handles the CC. Not to bias the bwtest results, the server handles a single client at a time. The total time for the test is estimated, and other clients are told for how long to wait if they arrive during a running test.

For each client request, the server establishes a new SCION UDP Data Connection (DC). For the traffic sent on this DC, the server uses the (reversed) path used by the client on the control channel.

The server starts sending right after it established the DC. Since the client already set up the receiving function, the server->client bwtest starts right away. The client only starts sending after it receives a successful server response.

The results are stored in a map, indexed by the client SCION address (ISD, AS, IP) plus the port number. To ensure that the correct results are returned, we also use the AES key of the client->server direction as identifier of the connection (to prevent an erroneous client who fetches the results too early to obtain the results of a previous run). If the results are requested too early, the server indicates how many additional seconds to wait until the results will be ready.
