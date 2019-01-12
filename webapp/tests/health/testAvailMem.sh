# test if the available memory is greater than 2G, fail if not

# An error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# get the available memory for this virtual machine
availMem=$(df | grep '/' -w | tr -s ' ' | cut -d ' ' -f4)

# test if the available memory is greater than 2G
if [ "$availMem" -lt 2097152 ]; then
    error_exit "Error: Available Memory less than 2G, please destroy your virtual machine and create a new one"
else
    echo "Test for available memory succeeds. Size of available memory: $((availMem / 1048576))G."
    exit 0
fi


