#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
ia=`cat ~/go/src/github.com/scionproto/scion/gen/ia`
echo "IA found: $ia"

isd=$(echo ${ia} | cut -d"-" -f1)

# check connection to local attachment point, default to ETHZ
case $isd in
    17|*)
        # 17-ffaa:0:1107 Attachment Point, ETHZ
        addr='192.33.93.177'
        ;;
    18)
        # 18-ffaa:0:1202 Attachment Point, WISC
        addr='www.wisc.edu'
        ;;
    19)
        # 19-ffaa:0:1303 Attachment Point, OVGU
        addr='inf.ovgu.de'
        ;;
    20)
        # 20-ffaa:0:1404 Attachment Point, KU
        addr='cs.korea.edu'
        ;;
esac

# if no response under default ping timeout consider connection failed
ping -c 1 -w 5 $addr &>/dev/null
if [ $? -ne 0 ] ; then
    echo "For ISD $ia, tried $addr, but it is unreachable by ping."
    exit 1
else
    echo "For ISD $ia, reached a connection to $addr by ping."
fi

exit $?
