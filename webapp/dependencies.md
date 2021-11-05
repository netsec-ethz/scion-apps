# Webapp Dependencies

## Package Default CL
```shell
systemctl cat scion-webapp
```

## Developer Default CL
```shell
webapp -h
```

## System Binaries Used
- bash
- echo
- cd
- sed
- date
- curl
- set
- grep
- python3
- df
- tr
- free
- sleep
- cat
- systemctl

## SCION Binaries Used/Checked (should be somewhere on $PATH)
- scion (for scion ping and scion traceroute)
- scion-bwtestclient
- scion-sensorfetcher

## Scripts/Directories Used/Checked
- $SCION_ROOT/scion.sh (only for local topologies (ISD < 16))
- $sgen/.*topology.json (checking all subdirectories of $sgen for topology.json)
- $sgen/.*(sciond|sd).toml (for reading sciond address)
- $sgen/*cs*.toml (for reading prometheus server address)
- $sgenc/*.crt
- $sgenc/*.trc
- $sgenc/ps[IA]-1.path.db
- /run/shm/dispatcher/default.sock

## Static Webserver Required
- $srvroot/favico.ico
- $srvroot/config/
- $srvroot/static/css/
- $srvroot/static/html/
- $srvroot/static/img/
- $srvroot/static/js/
- $srvroot/template/
- $srvroot/tests/health/

## Static Webserver Generated
- $srvroot/webapp.db
- $srvroot/data/
- $srvroot/logs/
