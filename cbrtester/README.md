# Constant Bit Rate Tester

Sends at a constant rate using the specified path.

The client chooses the packet size, the rate and the path.
The server prints out the receive rate and stall events (freezes or
interruptions in the received packet stream of more than 100ms) if any are
detected.
