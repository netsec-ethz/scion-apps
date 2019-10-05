# Webapp Dependencies

## Package Default CL
```shell
webapp \
-a 0.0.0.0 \
-p 8000 \
-r /var/lib/scion/webapp/web/data \
-srvroot /var/lib/scion/webapp/web \
-sabin /usr/bin \
-sroot /etc/scion \
-sbin /usr/bin \
-sgen  /etc/scion/gen \
-sgenc /var/lib/scion \
-slogs /var/log/scion
```

## Developer Default CL
```shell
webapp \
-a 127.0.0.1 \
-p 8000 \
-r . \
-srvroot $GOPATH/src/github.com/netsec-ethz/scion-apps/webapp/web \
-sabin $GOPATH/bin \
-sroot $GOPATH/src/github.com/scionproto/scion \
-sbin $GOPATH/src/github.com/scionproto/scion/bin \
-sgen $GOPATH/src/github.com/scionproto/scion/gen \
-sgenc $GOPATH/src/github.com/scionproto/scion/gen-cache \
-slogs $GOPATH/src/github.com/scionproto/scion/logs
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




