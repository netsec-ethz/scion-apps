module github.com/netsec-ethz/scion-apps

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/bclicn/color v0.0.0-20180711051946-108f2023dc84
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/kormat/fmt15 v0.0.0-20181112140556-ee69fecb2656
	github.com/kr/pty v1.1.8
	github.com/lucas-clemente/quic-go v0.17.3
	github.com/mattn/go-sqlite3 v1.9.1-0.20180719091609-b3511bfdd742
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/netsec-ethz/rains v0.1.0
	github.com/scionproto/scion v0.5.1-0.20200925081908-c8b0d56ce587
	github.com/smartystreets/goconvey v1.6.4
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/netsec-ethz/rains => github.com/marcfrei/rains v0.1.1-0.20200925121856-ef71d7eb3d84
