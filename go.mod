module github.com/netsec-ethz/scion-apps

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/bclicn/color v0.0.0-20180711051946-108f2023dc84
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/britram/borat v0.0.0-20181011130314-f891bcfcfb9b // indirect
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/inconshreveable/log15 v0.0.0-20161013181240-944cbfb97b44
	github.com/kormat/fmt15 v0.0.0-20181112140556-ee69fecb2656
	github.com/kr/pty v1.1.8
	github.com/lucas-clemente/quic-go v0.11.0
	github.com/mattn/go-sqlite3 v1.9.1-0.20180719091609-b3511bfdd742
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/netsec-ethz/rains v0.0.0-20190912114116-83f56a7cb2d1
	github.com/scionproto/scion v0.4.0
	github.com/smartystreets/goconvey v1.6.4
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/d4l3k/messagediff.v1 v1.2.1 // indirect
)

replace github.com/scionproto/scion => github.com/netsec-ethz/netsec-scion v0.0.0-20191125131751-ff10566d67ac

go 1.13
