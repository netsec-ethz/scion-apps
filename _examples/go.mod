module examples

go 1.16

require (
	github.com/gorilla/handlers v1.5.1
	github.com/lucas-clemente/quic-go v0.23.0
	github.com/netsec-ethz/scion-apps v0.5.1-0.20220504120040-79211109ed3f
	github.com/scionproto/scion v0.6.1-0.20220202161514-5883c725f748
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	inet.af/netaddr v0.0.0-20210903134321-85fa6c94624e
)

replace github.com/scionproto/scion => github.com/netsec-ethz/scion v0.6.1-0.20220908152129-737e910b8d60

replace github.com/netsec-ethz/scion-apps => ../
