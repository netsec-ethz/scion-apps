#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
ia=$(cat ~/go/src/github.com/scionproto/scion/gen/ia | sed "s/_/:/g")
echo "IA found: $ia"

ip=$(hostname -I | cut -d" " -f1)
echo "IP found: $ip"

# check connection to local attachment point, default to ETHZ
isd=$(echo ${ia} | cut -d"-" -f1)
case $isd in
    17|*)
        # 17-ffaa:0:1107 Attachment Point, ETHZ
        ia_dst='17-ffaa:0:1107'
        ip_dst='192.33.93.195'
        ;;
    18)
        # 18-ffaa:0:1202 Attachment Point, WISC
        ia_dst='18-ffaa:0:1202'
        ip_dst='128.105.21.208'
        ;;
    19)
        # 19-ffaa:0:1303 Attachment Point, OVGU
        ia_dst='19-ffaa:0:1303'
        ip_dst='141.44.25.144'
        ;;
    20)
        # 20-ffaa:0:1404 Attachment Point, KU
        ia_dst='20-ffaa:0:1404'
        ip_dst='203.230.60.98'
        ;;
esac

# if no response under default scmp ping timeout consider connection failed
cmd="$SC/bin/scmp echo -c 1 -timeout 5s -local $ia,[$ip] -remote $ia_dst,[$ip_dst]"
recv=$($cmd | grep -E -o '[0-9]+ received' | cut -f1 -d' ')
if [ "$recv" != "1" ]; then
    echo "SCMP echo failed. For ISD $ia, tried $ia_dst,[$ip_dst]."
    exit 1
else
    echo "SCMP echo succeeded. For ISD $ia, recieved response from $ia_dst,[$ip_dst]."
fi

exit $?
