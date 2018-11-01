#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# define threshold
min_sec=15

ntp_offset=$(ntpq -pn | \
     /usr/bin/awk 'BEGIN { offset=1000 } $1 ~ /\*/ { offset=$9 } END { print offset }')
echo "Network time offset is $ntp_offset seconds."

# compare ntp offset to threshold
off=$(awk 'BEGIN {print ("'${ntp_offset#-}'" > "'${min_sec}'")}')

if [ "$off" -eq "1" ]; then
    echo "Offset must be within $min_sec seconds."
    exit $off
fi

exit $?
