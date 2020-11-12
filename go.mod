module github.com/netsec-ethz/scion-apps

go 1.13

require (
	gitea.com/goftp/file-driver v0.0.0-20190812052443-efcdcba68b34
	github.com/BurntSushi/toml v0.3.1
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/bclicn/color v0.0.0-20180711051946-108f2023dc84
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/jlaffaye/ftp v0.0.0-20201021201046-0de5c29d4555
	github.com/kormat/fmt15 v0.0.0-20181112140556-ee69fecb2656
	github.com/kr/pty v1.1.8
	github.com/lucas-clemente/quic-go v0.18.1 // TODO make sure that the other apps work with this version too
	github.com/mattn/go-sqlite3 v1.9.1-0.20180719091609-b3511bfdd742
	github.com/msteinert/pam v0.0.0-20190215180659-f29b9f28d6f9
	github.com/netsec-ethz/rains v0.1.0
	github.com/scionproto/scion v0.5.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/stretchr/testify v1.6.1
	goftp.io/server v0.4.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
