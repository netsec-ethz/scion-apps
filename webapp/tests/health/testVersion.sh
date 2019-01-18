# Test if scion and scionlab version is up to date

# error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# check if variable $GOPATH is properly defined, we already know from previous health check that $SC is properly defined, so we ignore the test for $SC here
if [[ -d $GOPATH ]]
then
    echo "Variable \$GOPATH is set correctly."
else
    error_exit "Variable \$GOPATH is not properly set."
fi

# check if the scion and scion-apps repo exist and are up to date

# $1 is the repo path and $2 is the repo name
check_repo()
{
    if [[ ! -d $1 ]]
    then
        error_exit "Repo $2 doesn't exist."
    fi

    cd $1
    UPSTREAM='@{u}'
    LOCAL=$(git rev-parse @)
    REMOTE=$(git rev-parse "$UPSTREAM")
    BASE=$(git merge-base @ "$UPSTREAM")
    
    if [[ $LOCAL = $REMOTE ]]
    then
        echo "Git repo '$2' at path $1 is up-to-date"
    elif [[ $LOCAL = $BASE ]]
    then
        error_exit "Git repo '$2' at path $1 needs to pull"
    elif [[ $REMOTE = $BASE ]]
    then
        error_exit "Git repo '$2' at path $1 needs to push"
    else
        error_exit "Git repo '$2' at path $1 is diverged"
    fi
}

check_repo $SC scion
check_repo $GOPATH/src/github.com/netsec-ethz/scion-apps scion-apps 
