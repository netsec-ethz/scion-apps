#!/bin/bash
exit

[sig]
  # ID of the SIG (required)
  ID = "sigA"

  # The SIG config json file. (required)
  SIGConfig = "/home/ubuntu/go/src/github.com/scionproto/scion/gen/ISD${ISD}/AS${AS}/sig${IA}-1/sigA.json"

  # The local IA (required)
  IA = "${IAd}"

  # The bind IP address (required)
  IP = "10.0.8.A"

  # Control data port, e.g. keepalives. (default 10081)
  CtrlPort = 10081

  # Encapsulation data port. (default 10080)
  EncapPort = 10080

  # SCION dispatcher path. (default "")
  Dispatcher = ""

  # Name of TUN device to create. (default DefaultTunName)
  Tun = "sigA"

  # Id of the routing table (default 11)
  TunRTableId = 11

[sd_client]
  # Sciond path. It defaults to sciond.DefaultSCIONDPath.
  Path = "/run/shm/sciond/default.sock"

  # Maximum time spent attempting to connect to sciond on start. (default 20s)
  InitialConnectPeriod = "20s"

[logging]
[logging.file]
  # Location of the logging file.
  Path = "/home/ubuntu/go/src/github.com/scionproto/scion/logs/sig${IA}-1.log"

  # File logging level (trace|debug|info|warn|error|crit) (default debug)
  Level = "debug"

  # Max size of log file in MiB (default 50)
  # Size = 50

  # Max age of log file in days (default 7)
  # MaxAge = 7

  # How frequently to flush to the log file, in seconds. If 0, all messages
  # are immediately flushed. If negative, messages are never flushed
  # automatically. (default 5)
  FlushInterval = 5
[logging.console]
  # Console logging level (trace|debug|info|warn|error|crit) (default crit)
  Level = "debug"

[metrics]
# The address to export prometheus metrics on. (default 127.0.0.1:1281)
  Prometheus = "127.0.0.1:1282"
