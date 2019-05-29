# scion-netcat
A SCION port of the netcat process.


## Usage
```
./netcat <host> <port>
./netcat -l <port>
```

Remember to generate a TLS certificate first (this will generate them in the current working directory):
```
openssl req -newkey rsa:2048 -nodes -keyout ./key.pem -x509 -days 365 -out ./certificate.pem
```

See `./netcat -h` for more.

