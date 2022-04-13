module github.com/netsec-ethz/scion-apps

go 1.16

require (
	github.com/creack/pty v1.1.17
	github.com/gorilla/handlers v1.5.1
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/kormat/fmt15 v0.0.0-20181112140556-ee69fecb2656
	github.com/lucas-clemente/quic-go v0.23.0
	github.com/mattn/go-sqlite3 v1.14.4
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/netsec-ethz/rains v0.5.0
	github.com/pelletier/go-toml v1.9.4
	github.com/scionproto/scion v0.6.1-0.20220202161514-5883c725f748
	github.com/smartystreets/goconvey v1.6.7
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	inet.af/netaddr v0.0.0-20210903134321-85fa6c94624e
)

replace github.com/scionproto/scion => github.com/netsec-ethz/scion v0.6.1-0.20220407132138-98d64387e90d
