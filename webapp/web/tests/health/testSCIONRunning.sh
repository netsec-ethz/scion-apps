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
isd=$(echo ${iaFile} | cut -d"-" -f1)

sdaddress=$(echo $2)
echo "sciond address: $sdaddress"

# check if "./scion.sh status" returns anything, fail if it does
if [ $isd -ge 16 ]; then
    status="$(systemctl -t service --failed | grep scion*.service 2>&1)"
else
    # localhost testing
    [ -z "$SCION_ROOT" ] && error_exit "SCION_ROOT env variable not set, scion.sh can't be found"
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

# check TCP sciond socket is running; split host:port for netcat
cmd="nc -z $(echo "$sdaddress" | sed -e 's/\[\?\([^][]*\)\]\?:/\1 /')"
echo "Running: $cmd"
if $cmd 2>&1; then
    echo "SCIOND is listening on $sdaddress."
else
    error_exit "SCIOND did not respond on $sdaddress."
fi

echo "Test for SCION running succeeds."
