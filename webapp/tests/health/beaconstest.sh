#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# get local IA
ia=`cat ~/go/src/github.com/scionproto/scion/gen/ia`
echo "IA found: $ia"

# format log file and beacons grep string
fsafe_ia=$(echo $ia | sed "s/:/_/g")
logfile="~/go/src/github.com/scionproto/scion/logs/bs${fsafe_ia}-1.DEBUG"
echo "Log: $logfile"

# seek log entries for verified PCBs on today's date
regex_pcb="$(date +%Y-%m-%d).*Successfully verified PCB"
echo "Seeking regex: $regex_pcb"

count=$(grep -c "${regex_pcb}" \
    ~/go/src/github.com/scionproto/scion/logs/bs${fsafe_ia}-1.DEBUG)
echo "Verifications found on $(date +%Y-%m-%d): $count"

if (( count==0 )); then
    echo "No PCBs verified yet today."
    exit 1
fi

exit $?
