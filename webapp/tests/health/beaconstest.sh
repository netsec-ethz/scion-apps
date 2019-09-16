#!/bin/bash
# test will fail for non-zero exit and/or bytes in stderr

# timeout for reties to find vaerfied PCBs
timeout_ms=20000

# oldest accept age of last PCB
pcb_ms=10000

# allow IA via args, ignoring gen/ia
iaFile=$(echo $1 | sed "s/:/_/g")
echo "IA found: $iaFile"

# format log file and beacons grep string (was .DEBUG)
logfile=$SCION_LOGS/bs${iaFile}-1.log
echo "Log: $logfile"

# seek last log entry for verified PCBs (was "Successfully verified PCB")
regex_pcb="Registered beacons"
echo "Seeking regex: $regex_pcb"

epoch_s=$(date +"%s%6N")
diff_ms=$pcb_ms
while [ "$diff_ms" -ge $pcb_ms ]
do
    epoch_n=$(date +"%s%6N")

    # timeout after 20s of attempts
    diff_to=$((epoch_n-epoch_s))
    diff_to_ms=$((diff_to/1000))
    if (( diff_to_ms>timeout_ms )); then
        echo "Timeout: No PCBs verified in the last $((pcb_ms/1000)) seconds."
        exit 1
    fi

    # seek the last pcb verified, and determine age
    last_pcb=$(grep -a "${regex_pcb}" $logfile | tail -n 1)
    date=${last_pcb:0:32}
    epoch_l=$(date -d "${date}" +"%s%6N")
    diff=$((epoch_n-epoch_l))
    diff_ms=$((diff/1000))

    # if too old, wait 1s to try again
    if [ "$diff_ms" -ge 60000 ]; then
        sleep 1
    fi
done

echo "PCB verification found ${diff_ms}ms ago."
exit $?
