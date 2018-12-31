# run a few tests for the trouble shooting

echo "Test for available memory:"; bash testAvailMem.sh && echo "Test for VPN:"; bash testVPN.sh && echo "Test for SCION running:"; bash testSCIONRunning.sh && echo "All tests passed! Congratulations!"
