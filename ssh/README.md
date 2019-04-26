# SCION enabled SSH

SSH client and server running over SCION network.

# Installation

## Prerequisite

SCION infrastructure has to be installed and running. Instructions can be found [here](https://github.com/scionproto/scion)

Additional development library for PAM is needed:
```
sudo apt-get install libpam0g-dev
```

# Running

To generate TLS connection certificates:
```
# These are valid for 365 days, so you'll have to renew them periodically
# Client
cd ~/.ssh
openssl req -newkey rsa:2048 -nodes -keyout quic-conn-key.pem -x509 -days 365 -out quic-conn-certificate.pem
# Server
cd /etc/ssh
sudo openssl req -newkey rsa:2048 -nodes -keyout quic-conn-key.pem -x509 -days 365 -out quic-conn-certificate.pem
```

You'll also need to create a client key (if you don't have one yet):
```
cd ~/.ssh
ssh-keygen -t rsa -f id_rsa
```

And create an authorized key file for the server with the public key (note that you'd usually place this in `/home/<user>/.ssh/authorized_keys` whereas `<user>` is the user on the server you want to gain access to, but make sure not to overwrite an existing file):
```
cd $GOPATH/src/github.com/netsec-ethz/scion-apps/ssh/server
cp ~/.ssh/id_rsa.pub ./authorized_keys
```

Running the server:
```
cd $GOPATH/src/github.com/netsec-ethz/scion-apps/ssh/server
# If you are not root, you need to use sudo. You might also need the -E flag to preserve environment variables (like $SC)
sudo -E ./server -oPort=2200 -oAuthorizedKeysFile=./authorized_keys
# You might also want to disable password authentication for security reasons with -oPasswordAuthentication=no
```


Running the client:
```
cd $GOPATH/src/github.com/netsec-ethz/scion-apps/ssh/client
./client -p 2200 1-11,[127.0.0.1]
```

Using SCP (make sure you've done `chmod +x ./scp.sh` first):
```
cd $GOPATH/src/github.com/netsec-ethz/scion-apps/ssh/scp
./scp.sh -P 2200 localFileToCopy.txt [1-11,[127.0.0.1]]:remoteTarget.txt
```

