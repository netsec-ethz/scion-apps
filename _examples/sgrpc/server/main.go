package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/netip"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/quic-go/quic-go"
	"github.com/scionproto/scion/pkg/snet"
	"google.golang.org/grpc"

	pb "examples/sgrpc/proto"
)

type echoServer struct {
	pb.UnimplementedEchoServiceServer
}

var _ pb.EchoServiceServer = &echoServer{}

func (*echoServer) Echo(ctx context.Context,
	req *pb.EchoRequest) (*pb.EchoResponse, error) {
	resp := &pb.EchoResponse{
		Msg: req.Msg,
	}
	return resp, nil
}

var (
	ServerAddr = flag.String("server-addr", "127.0.0.1:5000", "Address the server should listen on")
)

func main() {
	flag.Parse()

	asCtx := pan.MustLoadDefaultASContext()

	addr, err := netip.ParseAddrPort(*ServerAddr)
	if err != nil {
		log.Fatalf("failed to parse server address")
	}

	echoServer := &echoServer{}
	grpcServer := grpc.NewServer()
	pb.RegisterEchoServiceServer(grpcServer, echoServer)

	tlsCfg := &tls.Config{
		Certificates: quicutil.MustGenerateSelfSignedCert(),
		NextProtos:   []string{"echo_service"},
	}

	localAddr := &snet.UDPAddr{
		IA:   asCtx.IA(),
		Host: net.UDPAddrFromAddrPort(addr),
	}
	quicListener, err := pan.ListenQUIC(context.Background(), asCtx, localAddr, tlsCfg, &quic.Config{}, nil)
	if err != nil {
		log.Fatalf("failed to listen SCION QUIC on %s: %v", *ServerAddr, err)
	}
	lis := quicutil.SingleStreamListener{QUICListener: quicListener}
	log.Println("listen on", quicListener.Addr())

	if err := grpcServer.Serve(&lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
