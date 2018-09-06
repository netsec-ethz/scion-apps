package main

import(
    "fmt"
    "io"
    "net"
    "encoding/binary"
    "log"
)

func readTime(r io.Reader) (uint64, error){
    buf := make([]byte, 8)  // uint64

    n, err := r.Read(buf[:])
    if err != nil {
        return 0, err
    }
    if(n!=8){
        return 0, fmt.Errorf("Expecting 8 byte reply!")
    }

    currentTime := binary.LittleEndian.Uint64(buf)
    log.Printf("Received time: %d", currentTime)
    return currentTime, nil
}

func GetTime(socketFile string) (uint64, error) {
    c, err := net.Dial("unix", socketFile)
    if err != nil {
        return 0, err
    }
    defer c.Close()

    return readTime(c)
}
