# check if SCION is running

# error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# check if "./scion.sh status" returns anything, fail if it does
cd $SC
status="$(bash $SC/scion.sh status 2>&1)"
if [[ $status ]]
then
    echo "hello"
    error_exit "Stop and start SCION again as following then retry the test:

ubuntu@ubuntu-xenial:~$ cd $SC
ubuntu@ubuntu-xenial:~/go/src/github.com/scionproto/scion$ ./scion.sh stop
Terminating this run of the SCION infrastructure
dispatcher: stopped
as17-ffaa_1_64:sd17-ffaa_1_64: stopped
as17-ffaa_1_64:br17-ffaa_1_64-1: stopped
as17-ffaa_1_64:cs17-ffaa_1_64-1: stopped
as17-ffaa_1_64:ps17-ffaa_1_64-1: stopped
as17-ffaa_1_64:bs17-ffaa_1_64-1: stopped
ubuntu@ubuntu-xenial:~/go/src/github.com/scionproto/scion$ ./scion.sh start
Compiling...
Running the network...
dispatcher: started
as17-ffaa_1_64:sd17-ffaa_1_64: started
as17-ffaa_1_64:br17-ffaa_1_64-1: started
as17-ffaa_1_64:bs17-ffaa_1_64-1: started
as17-ffaa_1_64:cs17-ffaa_1_64-1: started
as17-ffaa_1_64:ps17-ffaa_1_64-1: started

if the test still fails, please contact us and copy the following msg:
$status"
fi

# check if /gen and /gen/ia and /run/shm/sciond and /run/shm/dispatcher directories are present and if they contain a default.sock file, fail if not

# check if a directory $1 exists and if it contains a file $2
check_presence(){   
if [[ ! -d "$1" ]]
then
    error_exit "directory $1 doesn't exist, please contact us."
elif [[ ! -f "$1/$2" ]]
then
    error_exit "file $1/$2 doesn't exist, please contact us."
fi
}

check_presence $SC/gen ia
check_presence /run/shm/sciond default.sock
check_presence /run/shm/dispatcher default.sock

echo "Test for SCION running succeeds."	 
