# test if the available disk space is greater than 2G, fail if not

# An error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# get the available space for this virtual machine
availSpace=$(df | grep '/' -w | tr -s ' ' | cut -d ' ' -f4)

# test if the available disk space is greater than 2G
if [ "$availSpace" -lt 2048004 ]; then
    error_exit "Error: Available disk space less than 2G, please destroy your virtual machine and create a new one"
else
    echo "Test for available disk space succeeds. Size of available space: $((availSpace / 1024000))G."
    exit 0
fi

