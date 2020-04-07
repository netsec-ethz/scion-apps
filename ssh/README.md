# SCION enabled SSH

SSH client and server running over SCION network.

### Usage

SCION infrastructure has to be installed and running. Instructions can be found [here](https://netsec-ethz.github.io/scion-tutorials/)

You'll need to create a client key (if you don't have one yet):
```
cd ~/.ssh
ssh-keygen -t rsa -f id_rsa
```

And create an authorized key file for the server with the public key (note that you'd usually place this in `/home/<user>/.ssh/authorized_keys` whereas `<user>` is the user on the server you want to gain access to, but make sure not to overwrite an existing file):
```
cd scion-apps/ssh/server
cp ~/.ssh/id_rsa.pub ./authorized_keys
```

Running the server:
```
cd scion-apps/ssh/server
# If you are not root, you need to use sudo. You might also need the -E flag to preserve environment variables.
sudo -E ./server -oPort=2200 -oAuthorizedKeysFile=./authorized_keys
# You might also want to disable password authentication for security reasons with -oPasswordAuthentication=no
```


Running the client:
```
cd scion-apps/ssh/client
./client -p 2200 1-ffaa:1:abc,[127.0.0.1] -oUser=username
```

Using SCP:
```
cd scion-apps/ssh/scp
./scp.sh -P 2200 localFileToCopy.txt [1-ffaa:1:abc,[127.0.0.1]]:remoteTarget.txt
```

