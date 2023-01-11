package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"

	"google.golang.org/grpc"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"

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

	addr, err := netaddr.ParseIPPort(*ServerAddr)
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

	quicListener, err := pan.ListenQUIC(context.Background(), addr, nil, tlsCfg, nil)
	if err != nil {
		log.Fatalf("failed to listen SCION QUIC on %s: %v", *ServerAddr, err)
	}
	lis := quicutil.SingleStreamListener{Listener: quicListener}
	log.Println("listen on", quicListener.Addr())

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
