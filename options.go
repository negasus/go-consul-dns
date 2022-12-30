package go_consul_dns

import "time"

type Option func(r *ConsulResolver)

func WithConsulAddress(address string) Option {
	return func(r *ConsulResolver) {
		r.address = address
	}
}

func WithDatacenter(datacenter string) Option {
	return func(r *ConsulResolver) {
		r.datacenter = datacenter
	}
}

func WithDomain(domain string) Option {
	return func(r *ConsulResolver) {
		r.domain = domain
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(r *ConsulResolver) {
		r.timeout = timeout
	}
}
