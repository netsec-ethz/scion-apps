# test if the total memory is greater than 2G, fail if not

# An error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# get the total memory for this virtual machine
totalMem=$(free | grep 'Mem:' | tr -s ' ' | cut -d ' ' -f2)

# test if the total memory is greater than 2G
if [ "$totalMem" -lt 2048004 ]; then
     error_exit "Error: Total memory less than 2G, please contact us."
else
    echo "Test for total memory succeeds. Size of total memory: $((totalMem / 1024000))G."
    exit 0
fi


