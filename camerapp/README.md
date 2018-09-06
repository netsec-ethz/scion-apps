
# Documentation for camerapp application

This file contains information on the camerapp code itself. Check here
for [setup and installation
instructions](https://github.com/perrig/scionlab/blob/master/README.md).

The camerapp application is based on the SCION UDP socket written in
Go. Since UDP is not reliable, we construct a very simple reliability
protocol, where the client simply re-fetches blocks it has not
received. No reliability is implemented on the server side, it simply
delivers image file names or file blocks upon request.

## Wireline data format

List of commands:
* L: lists image available for download
     > request format: 1 byte "L"
	 >
     > response format:  1 byte "L", 1 byte filename length, filename string, int32 image length
* G: fetches a range of bytes from the file of the image
     > request format: 1 byte "G", 1 byte filename length, filename string, int32 starting byte, int32 ending byte
	 >
     > response format: 1 byte "G", int32 starting byte, int32 ending byte, bytes of image

Note: The int32 are in little endian format. The ending byte is not included in the response, so 0-1000 fetches [0:999].

## imagefetcher code

The imagefetcher code uses two different approaches for reliability.

* For fetching the most recent image name (by sending the "L" or list command to the server), we use the `SetReadDeadline` call to the UDP socket, then the ReadFrom call:
```go
err = udpConnection.SetReadDeadline(time.Now().Add(maxWaitDelay))
n, _, err = udpConnection.ReadFrom(packetBuffer)
```

Tip. After setting a read deadline, if it didn't trigger don't forget to clear it with a zero timestamp.
```go
var tzero time.Time // initialized to "zero" time
err = udpConnection.SetReadDeadline(tzero)
```
Otherwise, it will trigger during a subsequent `Read` call.

If the request or server response packet is lost, the ReadFrom call returns after `maxWaitDelay` and the application re-sends the request up to `maxRetries` number of times.

For fetching the actual image data, two goroutines are started, one for requesting blocks and one for receiving blocks. In this setup, the reliability is achieved through a select call where one case contains a timeout:
```go
select {
case k := <-receivedBlockChan:
	 ... // block k was received
case <-time.After(waitDuration):
	 ... // A timeout occurred
}
```

This setup enables us to use a window-based approach, where multiple file block requests are sent simultaneously. At any instant, the client can send requests for up to `maxNumBlocksRequested` blocks. The `requestedBlockMap` data structure keeps track of the blocks that were requested but have not yet been received.

## imageserver code

The imageserver code is quite simple. One goroutine periodically looks at the file system to detect if a new image appears. The read time of the image is recorded. After `MaxFileAge` time, the image is deleted from the file system, assuming a camera application that keeps depositing images.

The application contains a simple loop that waits for client requests to list the most recent file ("L") or want to get a block ("G").
