#!/bin/bash

mkdir ${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1
touch ${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.json
file=${SCION_GEN}/ISD${ISD}/AS${AS}/sig${IA}-1/sig${IA}.json
cat > file <<EOF
{
    "ASes": {
        "${IAdRemote}": {
            "Nets": [
                "172.16.${IdRemote}.0/24"
            ],
            "Sigs": {
                "remote-1": {
                    "Addr": "${IpRemote}",
                    "CtrlPort": ${CtrlPortRemote},
                    "EncapPort": ${EncapPortRemote}
                }
            }
        }
    },
    "ConfigVersion": 9001
}
EOF

cat $file
