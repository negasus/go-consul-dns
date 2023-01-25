package go_consul_dns

import "time"

// Option is init options type
type Option func(r *ConsulResolver)

// WithConsulAddress allows to redefine consul DND address
func WithConsulAddress(address string) Option {
	return func(r *ConsulResolver) {
		r.address = address
	}
}

// WithDatacenter allows to redefine consul datacenter value
func WithDatacenter(datacenter string) Option {
	return func(r *ConsulResolver) {
		r.datacenter = datacenter
	}
}

// WithDomain allows to redefine consul domain value
func WithDomain(domain string) Option {
	return func(r *ConsulResolver) {
		r.domain = domain
	}
}

// WithTimeout allows to redefine TCP timeouts (connection, write, read)
func WithTimeout(timeout time.Duration) Option {
	return func(r *ConsulResolver) {
		r.timeout = timeout
	}
}

// WithLogger allows to define logger
func WithLogger(logger Logger) Option {
	return func(r *ConsulResolver) {
		r.logger = logger
	}
}

// WithMaxRequestAttempts allows to redefine max request attempts if requests are fails
func WithMaxRequestAttempts(n int) Option {
	return func(r *ConsulResolver) {
		r.requestAttempts = n
	}
}

// WithGetAddressFromSRV allows to define get service address from SRV record and do not send A requests
func WithGetAddressFromSRV() Option {
	return func(r *ConsulResolver) {
		r.getAddressFromSRV = true
	}
}
