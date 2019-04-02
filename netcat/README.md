# scion-netcat
A repository containing the netcat process. Was initially contained in N2D4/scion-ssh, now has its own repo


## Get Started
Clone this repository, cd to it, install dependencies and build:
```govendor init
govendor add +e
govendor fetch +m
go build
```

Then, run it:
```./scion-netcat <host> <port>```

`-l` flag is not currently supported.

