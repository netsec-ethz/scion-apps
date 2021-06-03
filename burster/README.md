# Burster

Sends and receives gapless bursts of packets in 5s intervals.

The client chooses the packet size, the burst size and the path, and the server
prints out the number of received packets per burst. If any loss occurred then the number of received packets will be lower than the burst size set on the client.