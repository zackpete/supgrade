# supgrade

## Installation

```
go install github.com/zackpete/supgrade@latest
```

## Help

```
Usage of supgrade:
  -d string
        forwarding destination
  -n string
        nameserver to use for looking up destination
  -p int
        port to listen on (default 80)
  -t duration
        timeout for network operations (default 10s)
  -v    verbose errors
```

## Description

Created mainly to intercept HTTP traffic with tools like Wireshark
(for connections where one can't easily expose the TLS seesion key.)
