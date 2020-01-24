#!/bin/bash

mkdir ${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1
file=${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.config
cat >$file <<EOL
[sig]
  # ID of the SIG (required)
  ID = "sig${IA}"

  # The SIG config json file. (required)
  SIGConfig = "${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.json"

  # The local IA (required)
  IA = "${IAd}"

  # The bind IP address (required)
  IP = "172.16.0.${IdLocal}"

  # Control data port, e.g. keepalives. (default 10081)
  CtrlPort = ${CtrlPortLocal}

  # Encapsulation data port. (default 10080)
  EncapPort = ${EncapPortLocal}

  # SCION dispatcher path. (default "")
  Dispatcher = ""

  # Name of TUN device to create. (default DefaultTunName)
  Tun = "sig${IA}"

  # Id of the routing table (default 11)
  TunRTableId = ${IdLocal}

[sd_client]
  # Sciond path. It defaults to sciond.DefaultSCIONDPath.
  Path = "/run/shm/sciond/default.sock"

  # Maximum time spent attempting to connect to sciond on start. (default 20s)
  InitialConnectPeriod = "20s"

[logging]
[logging.file]
  # Location of the logging file.
  Path = "${SCION_LOGS}/sig${IA}-1.log"

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
EOL
cat $file
