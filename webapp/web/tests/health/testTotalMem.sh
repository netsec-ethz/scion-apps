# test if the total memory is greater than 2GB, fail if not

# An error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# NOTE: Since 'free' reports memory in base-2, and may have some kb
# trimmed off the total for video and other allocations, we will measure in
# base-10 to ensure we have at least 98% of the memory we require and still
# maintain readability in this script. We allocated 2048000kb for each VM,
# however the total memory can be altered as virtualbox demands.

# get the total memory for this virtual machine
totalMem=$(free | grep 'Mem:' | tr -s ' ' | cut -d ' ' -f2)
echo "Size of total memory: $((totalMem / 1000000))GB."

# test if the total memory is greater than 2GB
if [ "$totalMem" -lt 2000000 ]; then
     error_exit "Error: Total memory less than 2GB, please contact us."
else
    echo "Test for total memory succeeds."
    exit 0
fi
