# Roughtime port for SCION network

## About

Roughtime is a project that aims to provide secure time synchronisation. More information on the project can be found on the [original repository](https://roughtime.googlesource.com/roughtime)

This implementation also allows users who have a [ThinkerForge](https://www.tinkerforge.com/en/) board with [GPS 2](https://www.tinkerforge.com/de/blog/gps-bricklet-20-is-now-available/) and [RTC](https://www.tinkerforge.com/en/shop/real-time-clock-bricklet.html) bricklet to use accurate GPS time.

## Build

Install roughtime library:

```
cd $GOPATH/src/
git clone https://roughtime.googlesource.com/roughtime roughtime.googlesource.com
```

Build server:

```
cd $GOPATH/src/github.com/perrig/scionlab/roughtime/timeserver
go get
go build
```

Build client:

```
cd $GOPATH/src/github.com/perrig/scionlab/roughtime/timeclient
go get
go build
```

## Running the project

Running this project consists of two phases

1. Generating server(s) configuration and running the server
2. Register server's configuration on client and running client

### Step one - Generate server configuration

Roughtime protocol requires server to have public-key signature, so necessary keypair must be generated with following command:

```
cd timeserver
./timeserver configure <SCION_ADDRESS> --private_key <PATH_TO_PRIVATE_KEY> --config_file <PATH_TO_SERVER_CONFIGURATION> --name <SERVER_NAME>
```

Example for running command:

```
./timeserver configure 2-12,[127.0.0.1]:2233 --private_key p.key --config_file server.json --name server
```

will generate server configuration file `server.json` that looks like this:

```
{
  "name": "server1",
  "publicKeyType": "ed25519",
  "publicKey": "LIuImI0iDkFsyjtmwPtJX6wXwsMTuuDsCJW+zqQjPzo=",
  "addresses": [
    {
      "protocol": "udp4",
      "address": "2-12,[127.0.0.1]:2233"
    }
  ]
}
```

and file `p.key` that will contain servers private key.

#### Running server can be done by running following command:

```
./timeserver run <PATH_TO_PRIVATE_KEY> <PATH_TO_SERVER_CONFIGURATION>
```



### Step two - configure client

In order to run client it is necessary to register available roughtime servers. This is done by creating a json file and copying configuration from multiple servers into it as an array, example of such a configuration file can be seen below:

```
{
    "servers":[
        {
          "name": "server1",
          "publicKeyType": "ed25519",
          "publicKey": "30suULz9FakYxDXlZA2SItJ+0KO6OC+/MP1Dx7qnkxk=",
          "addresses": [
            {
              "protocol": "udp4",
              "address": "1-11,[127.0.0.1]:2233"
            }
          ]
        },
        {
          "name": "server2",
          "publicKeyType": "ed25519",
          "publicKey": "ra/l2mVx6Bynqo8VQQyvDhTnlWpZ4nkeoYRp0Qpb1NA=",
          "addresses": [
            {
              "protocol": "udp4",
              "address": "1-12,[127.0.0.1]:2233"
            }
          ]
        }
    ]
}
```

This configuration file can contain arbitrary number of servers.

#### Running the client

Running client is done by specifying client's SCION address and path to the file containing list of available servers.

```
./timeclient --address <CLIENTS_SCION_ADDRESS> --servers <PATH_TO_SERVERS_CONFIGURATION>
```

## Running GPS time daemon

If necessary Thinkerforge hardware is available, [Brick Daemon](https://www.tinkerforge.com/en/doc/Software/Brickd.html) is running and necessary python library [ntplib](https://pypi.python.org/pypi/ntplib/) is installed, additional script `timed.py` in `timeserver/gps_time_daemon` can be started with following command:

```
sudo ./timed.py
```

This script will use GPS time to update system time, which is used by roughtime server. 

Time daemon uses 3 sources of time:

- GPS time
- Time from RTC clock
- Time obtained from NTP

Every time GPS time is received it is compared with other time sources to verify they are close to each other. If its impossible to match GPS time with either RTC or NTP time, time won't be updated. (Next version will uze OLED display and buzzer to notify the user of situation).
