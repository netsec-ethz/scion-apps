module github.com/netsec-ethz/scion-apps

go 1.16

require (
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/bclicn/color v0.0.0-20180711051946-108f2023dc84
	github.com/gorilla/handlers v1.5.1
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/kormat/fmt15 v0.0.0-20181112140556-ee69fecb2656
	github.com/kr/pty v1.1.8
	github.com/lucas-clemente/quic-go v0.19.2
	github.com/mattn/go-sqlite3 v1.9.1-0.20180719091609-b3511bfdd742
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/netsec-ethz/rains v0.2.0
	github.com/pelletier/go-toml v1.8.1-0.20200708110244-34de94e6a887
	github.com/scionproto/scion v0.6.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/stretchr/testify v1.5.1
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/scionproto/scion => github.com/netsec-ethz/scion v0.0.0-20210209094654-fbc7c985966b
