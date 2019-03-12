#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
ia=$(cat ~/go/src/github.com/scionproto/scion/gen/ia)
echo "IA found: $ia"

# check connection to local attachment point, default to ETHZ
isd=$(echo ${ia} | cut -d"-" -f1)
case $isd in
    17|*)
        # 17-ffaa:0:1107 Attachment Point, ETHZ
        ip_dst='192.33.93.195'
        ;;
    18)
        # 18-ffaa:0:1202 Attachment Point, WISC
        ip_dst='128.105.21.208'
        ;;
    19)
        # 19-ffaa:0:1303 Attachment Point, OVGU
        ip_dst='141.44.25.144'
        ;;
    20)
        # 20-ffaa:0:1404 Attachment Point, KU
        ip_dst='203.230.60.98'
        ;;
esac

# if no response under default icmp ping timeout consider connection failed
cmd="ping -c 1 -w 5 $ip_dst"
recv=$($cmd | grep -E -o '[0-9]+ received' | cut -f1 -d' ')
if [ $recv -eq 0 ]; then
    echo "ICMP ping failed. For ISD $ia, tried $ip_dst."
    exit 1
else
    echo "ICMP ping succeeded. For ISD $ia, recieved response from $ip_dst."
fi
exit $?
