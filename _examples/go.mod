module examples

go 1.16

require (
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/handlers v1.5.1
	github.com/netsec-ethz/scion-apps v0.5.1-0.20220504120040-79211109ed3f
	github.com/quic-go/quic-go v0.34.0
	github.com/scionproto/scion v0.6.1-0.20220202161514-5883c725f748
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.28.0
	inet.af/netaddr v0.0.0-20230525184311-b8eac61e914a
)

replace github.com/scionproto/scion => github.com/netsec-ethz/scion v0.6.1-0.20220929101513-2408583f35d1

replace github.com/netsec-ethz/scion-apps => ../
