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

## SCION Binaries Used/Checked
- $sbin/scmp
- $sabin/bwtestclient
- $sabin/imagefetcher
- $sabin/sensorfetcher

## Scripts/Directories Used/Checked
- $sroot/scion.sh (deprecated)
- $slogs/bs[IA]-1.log
- $sgen/ISD*/AS* (scanning for local IAs)
- $sgen/ISD[ISD]/AS[AS]/endhost/topology.json
- $sgenc/*.crt
- $sgenc/*.trc
- $sgenc/ps[IA]-1.path.db
- /run/shm/dispatcher/default.sock
- /run/shm/sciond/default.sock

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
- $srvroot/data/images/
- $srvroot/logs/




