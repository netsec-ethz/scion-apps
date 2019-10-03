# test if the VPN configuration is correct, fail if not

set -e
# error exit function
error_exit()
{
    echo "$1" 1>&2
    exit 1
}

# test if this AS uses VPN, exit directly if not.
if [ ! -f /etc/openvpn/client.conf ]
then
    echo "VPN is not configured for this AS."
    exit 0
fi

# lines describing the tun0 interface
targetLines=$(ip address show dev tun0)

### 1.check if tun0 interface is present
if [ $(echo "$targetLines" | wc -l) -eq 1 ]
then
    error_exit "You are probably behind a firewall that does not allow UDP traffic on port 1194. Please check your /var/log/syslog to find out if there had been a timeout while trying to establish the openvpn connection (search for ovpn-client in the /var/log/syslog file). If you find out that the tun0 interface was not brought up because timeouts between your client and the VPN server, it is an indication that a firewall is filtering the traffic: please contact your IT service to add an exception for your machine and port 1194."
fi

# ip address of the tun0 interface
ipAddress=$(echo "$targetLines" | grep -oE "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b" | head -n1)
topologyFile=$SCION_GEN/ISD*/AS*/endhost/topology.json
# ip address specified in the topology file. If "bind" parameter present, use that one; "public" if not
ipTopology=$(cat $topologyFile | python3 -c "import sys, json
brs = json.load(sys.stdin)['BorderRouters']
interfaces=next(iter(brs.values()))['Interfaces']
inter=next(iter(interfaces.values()))
print(inter['BindOverlay'] if 'BindOverlay' in inter.keys() else inter['PublicOverlay']['Addr'])")

# 2.check if the ip address from the tun0 interface is consistent with the one from the topology.
if [[ $ipAddress != $ipTopology ]]; then
    error_exit "The tun0 IP address doesn't match the IP address in your topology file, please destroy the existing virtual machine and remove its settings by first logging out of it and then running the steps described in the snippet vagrant destroy. After destroying the virtual machine, we can delete its configuration:

$ vagrant destroy -f
.
.
.
$ cd ..
$ pwd
/home/user/Downloads/
$ rm -r user@example.com_17-ffaa_1_64
Now check in the Coordinator webpage that your AS is correctly attached to your AP of choice, and that you are using the right tarball file. If in doubt, you can always click on Re-download my SCIONLab AS Configuration to get it again. Re-download does not configure the AS, but returns the latest configuration the Coordinator has for it. Wait 15 minutes (the reason being sometimes the attachment point needs 15 minutes to process your request). You should have received an email stating the success of your request. In the hopefully successful state, start again from the checking tarbal step. If after waiting these 15 minutes you did not receive the success email, or you received it but still don't see the same IP address in the tun0 interface as in the topology file, contact us."
fi

echo "VPN tun0 interface found at $ipAddress matching binding or public address"
echo "Test for VPN succeeds."
