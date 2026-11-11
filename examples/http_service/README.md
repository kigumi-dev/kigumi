# http_service

An HTTP service and its client in one program, using `std/net/http` on
loopback.

## Prerequisites

Loopback TCP (`requires: network-loopback`). No TLS fixture is needed: the
example serves plain HTTP on `127.0.0.1` port 0, and `std/net/tls` is out of
scope here.

## Run

```
kigumi run examples/http_service
kigumi build examples/http_service -o http_service && ./http_service
```

The port is chosen by the OS and never printed, so the output is stable.

## Test

```
kigumi test examples/http_service
```

The run itself is the check: the client exercises routing, middleware, JSON,
status codes, keep-alive/close, a timeout and shutdown before printing.

## Expected

`expect.txt` holds the exact status lines, selected headers (stable ones such
as `X-Request-Id`, `Allow`, `ETag`) and bodies for the GET/POST/404/405/204/
304/HEAD cases, the timeout, and the shutdown line.

## Next

`secret` moves to another explicit-boundary subsystem: constant-time code.
