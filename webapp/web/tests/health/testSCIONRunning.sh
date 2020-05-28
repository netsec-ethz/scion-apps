# check if SCION is running

# error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# allow IA via args
iaFile=$(echo $1 | sed "s/:/_/g")
echo "IA found: $iaFile"

sdaddress=$(echo $2)
echo "sciond address: $sdaddress"

isd=$(echo ${iaFile} | cut -d"-" -f1)

# check if "./scion.sh status" returns anything, fail if it does
if [ $isd -ge 16 ]; then
    status="$(systemctl -t service --failed | grep scion*.service 2>&1)"
else
    # localhost testing
    cd $SCION_ROOT
    status="$($SCION_ROOT/scion.sh status 2>&1)"
fi

if [[ $status ]]
then
    echo "SCION status has reported a problem: $status."
    error_exit "Stop and start SCION again as following then retry the test:

$ cd \$SC
$ ./scion.sh stop
$ ./scion.sh start
$ ./scion.sh status

if the test still fails, please contact us and copy the following msg:
$status"
else
    echo "SCION running status is normal."
fi

# check if /gen and /gen/ia and /run/shm/sciond and /run/shm/dispatcher directories are present and if they contain a default.sock file, fail if not

# check if a directory $1 exists and if it contains a file $2
check_presence(){
if [[ ! -d "$1" ]]
then
    error_exit "Directory $1 doesn't exist, please contact us."
else
    echo "Directory $1 found."
fi

if [[ ! -e "$1/$2" ]]
then
    error_exit "File $1/$2 doesn't exist, please contact us."
else
    echo "File $1/$2 found."
fi
}

check_presence /run/shm/dispatcher default.sock

if [ $isd -ge 16 ]; then
    # not used for localhost testing
    cmd="curl -m 1 $sdaddress/config"
    echo "Running: $cmd"
    recv=$($cmd | grep -E -o '[0-9]+ bytes received' | cut -f1 -d' ')
    echo $recv
    if [ "$recv" == "0" ]; then
        error_exit "SCIOND did not respond from $sdaddress/config."
    else
        echo "SCIOND responded."
    fi
fi

echo "Test for SCION running succeeds."
