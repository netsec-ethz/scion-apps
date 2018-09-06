
# bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network. This document describes the design of the code and protocol. Instructions on the installation and usage are described in the main [README.md](https://github.com/perrig/scionlab/blob/master/README.md).

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

The client application reads the command line parameters and establishes two SCION UDP connections to the bwtestserver: a Control Connection (CC) and a Data Connection (DC). The port numbers for the DC are simply picked as one larger than the respective ports of the CC (the CC port numbers are passed on the command line). (Note: if the application is executed locally, the client and server port numbers should be picked with a difference of at least 2, otherwise the same local port numbers would be used which results in an error.)

To achieve reliability for the initial request, the SetReadDeadline function is used. If the server responds with a number of seconds to wait, that amount of time is waited off before another request is sent (as the server only serves a single client at a time). Reliability for fetching the results is achieved in the same way.

## bwtestserver

The server runs a main loop that handles the CC. Not to bias the bwtest results, the server handles a single client at a time. The total time for the test is estimated, and other clients are told for how long to wait if they arrive during a running test.

For each client request, the server establishes a new SCION UDP connection to the client. For this, the server needs to perform a path lookup, so the path client->server may be different from the path server->client for the DC. In some rare cases, the server path lookup may fail, which results in an error message that is sent to the client, encouraging the client to try again in 1 second.

The server starts sending right after it established the DC. Since the client already set up the receiving function, the server->client bwtest starts right away. The client only starts sending after it receives a successful server response.

To estimate the running time, sending and receiving time estimates are computed. From the server's perspective, since there is uncertainty for the running time of the client->server bwtest, the estimate is updated after the first packet is received.

Instead of using channels to synchronize the main loop with the sending and receiving functions, we make use of the time estimate and the value of the results, where a positive value for the number of packets counted indicates that the receiving has been completed. Since there is no uncertainty on the completion of the sending function, the receiving function will close the DC.

The results are stored in a map, indexed by the client SCION address (ISD, AS, IP) plus the port number. The goroutine `purgeOldResults` takes care of deleting results that are older than 1 minute. To ensure that the correct results are returned, we also use the AES key of the client->server direction as identifier of the connection (to prevent an erroneous client who fetches the results too early to obtain the results of a previous run). If the results are requested too early, the server indicates how many additional seconds to wait until the results will be ready.

***
