# pastehub

`pastehub` is a small in-memory shared paste buffer for Tailscale networks.

## Features

- single binary for server and client
- binary-safe payloads up to a configured max size
- named buffers
- one current item per buffer
- HTTP API
- stdin/stdout client workflows
- default buffer when no buffer name is provided
- in-memory only, no persistence
- no auth
- access allowed only from loopback or Tailscale IPs

## Build

```bash
make build
```

The binary is written to `bin/pastehub`.

## Test

```bash
make test
```

## Run the server

```bash
./bin/pastehub serve --listen :8080 --max-bytes 1048576
```

## Client examples

Set the default buffer from stdin:

```bash
echo hello | ./bin/pastehub set
```

Set a named buffer from a file:

```bash
./bin/pastehub set work --file ./payload.bin --type application/octet-stream
```

Get a buffer to stdout:

```bash
./bin/pastehub get work > out.bin
```

Get the default buffer to a file:

```bash
./bin/pastehub get --out out.bin
```

List buffers:

```bash
./bin/pastehub list
```

Delete a buffer:

```bash
./bin/pastehub delete work
```

Use a different server:

```bash
./bin/pastehub get --server http://100.x.y.z:8080
```

or via environment variable:

```bash
export PASTEHUB_SERVER=http://100.x.y.z:8080
./bin/pastehub list
```

## HTTP API

Base path: `/v1`

- `PUT /v1/buffers/{name}`: store raw request body in a buffer
- `GET /v1/buffers/{name}`: fetch raw buffer contents
- `HEAD /v1/buffers/{name}`: fetch metadata via headers only
- `DELETE /v1/buffers/{name}`: clear a buffer
- `GET /v1/buffers/{name}/meta`: fetch JSON metadata
- `GET /v1/buffers`: list buffers and metadata

### Useful request headers on `PUT`

- `Content-Type`
- `X-Item-Name`

### Useful response headers on `GET` and `HEAD`

- `Content-Type`
- `Content-Length`
- `ETag`
- `X-Buffer-Name`
- `X-Item-Name`
- `X-Item-Size`
- `X-Item-SHA256`
- `X-Created-At`
