# test if the available space and total memory are greater than 2G, fail if not

# An error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# get the available space for this virtual machine
availSpace=$(df | grep '/' -w | tr -s ' ' | cut -d ' ' -f4)

# get the total memory for this virtual machine
totalMem=$(free | grep 'Mem:' | tr -s ' ' | cut -d ' ' -f2)

# test if the available space and total memory is greater than 2G
if [ "$availSpace" -lt 2048004 ]; then
    error_exit "Error: Available space less than 2G, please destroy your virtual machine and create a new one"
elif [ "$totalMem" -lt 2048004 ]; then
     error_exit "Error: Total memory less than 2G, please contact us"
else
    echo "Test for available space and total memory succeeds. Size of available space: $((availSpace / 1024000))G. Size of total memory: $((totalMem / 1024000))G."
    exit 0
fi


