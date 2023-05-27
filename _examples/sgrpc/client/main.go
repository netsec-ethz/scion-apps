package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/quic-go/quic-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"inet.af/netaddr"

	pb "examples/sgrpc/proto"
)

var (
	Message    = flag.String("message", "", "Message to send to the gRPC echo server")
	ServerAddr = flag.String("server-addr", "1-ff00:0:111,127.0.0.1:5000", "Address of the echo server")
)

func NewPanQuicDialer(tlsCfg *tls.Config) func(context.Context, string) (net.Conn, error) {
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		panAddr, err := pan.ResolveUDPAddr(ctx, addr)
		if err != nil {
			return nil, err
		}

		clientQuicConfig := &quic.Config{KeepAlive: true}
		session, err := pan.DialQUIC(ctx, netaddr.IPPort{}, panAddr, nil, nil, "", tlsCfg, clientQuicConfig)
		if err != nil {
			return nil, fmt.Errorf("did not dial: %w", err)
		}
		return quicutil.NewSingleStream(session)
	}

	return dialer
}

func main() {
	flag.Parse()

	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"echo_service"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	grpcDial, err := grpc.DialContext(ctx, *ServerAddr,
		grpc.WithContextDialer(NewPanQuicDialer(tlsCfg)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to dial gRPC: %v", err)
	}

	c := pb.NewEchoServiceClient(grpcDial)

	req := &pb.EchoRequest{
		Msg: *Message,
	}
	resp, err := c.Echo(ctx, req)
	if err != nil {
		log.Fatalf("gRPC did not connect: %v", err)
	}

	fmt.Println(resp.Msg)
}
