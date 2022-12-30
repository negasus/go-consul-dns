# Go-Consul-DNS

This library allows you to receive services addresses from Consul DNS over tcp protocol.

By default, DNS works over udp. This can lead to confusion for large responses.

For example:

Your consul has 300 services, and you want to receive SRV records with next go code:

```go
r := &net.Resolver{
    Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
        return net.Dial(network, "127.0.0.1:8600")
    },
}

_, srv, err := r.LookupSRV(context.Background(), "myservice", "tcp", "service.dc1.consul")
if err != nil {
    panic(err)
}

fmt.Printf("%d\n", len(srv))
```

Surprise, len can be equals ...26. But why? 

First, try to receive SRV records with `dig`

```sh
dig @127.0.0.1 -p 8600 myservice.service.dc1.consul. SRV
```

All ok, we saw 300 records.

> TODO
 
## Usage

```sh
go get -u github.com/negasus/go-consul-dns
```

```go
package main

import (
	"fmt"
	consuldns "github.com/negasus/go-consul-dns"
)

func main() {
	r, _ := consuldns.New("myservice")
	defer r.Close()

	r.Update()

	addresses := r.All()

	fmt.Printf("%v", addresses) // addresses is []string with all service addresses
}
```

## API

### `New(serviceName string, opts ...Option) (*ConsulResolver, error)`

Creates new Resolver, connect to consul DNS

### `Update() error`

Receive new SRV records, parse and store service addresses to the cache

### `All() []string`

Get all addresses from cache. It will be empty, if you do not call `Update`

### `Next() string`

Get next address from the cache with simple round-robin

### `Random() string`

Get random address from the cache

### `Close() error`

Close connection to the consul

## Options

### `WithConsulAddress(address string)`

> Default: `127.0.0.1:8600`

Redefine consul address

### `WithDatacenter(datacenter string)`

> Default: `dc1`

Redefine datacenter name

### `WithDomain(domain string)`

> Default: `consul`

Redefine domain

### `WithTimeout(timeout time.Duration)`

> Default: `10 seconds`

Redefine read/write connection timeout

Example:

```go
r := New("myservice", WithConsulAddress("127.0.0.1:8800"), WithDatacenter("dc-10"))
```