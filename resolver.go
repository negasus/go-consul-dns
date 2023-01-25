package go_consul_dns

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type Logger interface {
	Printf(format string, a ...any)
}

type nopLogger struct{}

func (l *nopLogger) Printf(_ string, _ ...any) {}

var (
	defaultTimeout         = time.Second * 10
	defaultConsulAddress   = "127.0.0.1:8600"
	defaultDatacenter      = "dc1"
	defaultDomain          = "consul"
	defaultRequestAttempts = 16
)

type Resolver interface {
	Update() error
	All() []string
	Random() string
	Next() string
	Close() error
}

type ConsulResolver struct {
	address           string
	datacenter        string
	domain            string
	timeout           time.Duration
	getAddressFromSRV bool
	requestAttempts   int
	logger            Logger

	mx       *sync.RWMutex
	counter  int64
	data     []string
	inUpdate int64

	connsPool *sync.Pool
	dnsName   dnsmessage.Name
}

func New(service string, opts ...Option) (*ConsulResolver, error) {
	r := &ConsulResolver{
		address:         defaultConsulAddress,
		datacenter:      defaultDatacenter,
		domain:          defaultDomain,
		timeout:         defaultTimeout,
		requestAttempts: defaultRequestAttempts,
		mx:              &sync.RWMutex{},
		connsPool:       &sync.Pool{},
		logger:          &nopLogger{},
	}

	for _, o := range opts {
		o(r)
	}

	var err error

	r.dnsName, err = dnsmessage.NewName(service + ".service." + r.datacenter + "." + r.domain + ".")
	if err != nil {
		return nil, fmt.Errorf("error parse service name, %w", err)
	}

	return r, nil
}

func (r *ConsulResolver) All() []string {
	r.mx.RLock()
	defer r.mx.RUnlock()

	return r.data
}

func (r *ConsulResolver) Random() string {
	r.mx.RLock()
	defer r.mx.RUnlock()

	if len(r.data) == 0 {
		return ""
	}

	return r.data[rand.Intn(len(r.data))]
}

func (r *ConsulResolver) Next() string {
	r.mx.RLock()
	defer r.mx.RUnlock()

	if len(r.data) == 0 {
		return ""
	}

	n := atomic.AddInt64(&r.counter, 1)
	return r.data[int(n-1)%len(r.data)]
}

func (r *ConsulResolver) Close() {
	for {
		c := r.connsPool.Get()
		if c == nil {
			return
		}
		errClose := c.(net.Conn).Close()
		if errClose != nil {
			r.logger.Printf("error close connection, %v", errClose)
		}
	}
}

func (r *ConsulResolver) Update() error {
	if !atomic.CompareAndSwapInt64(&r.inUpdate, 0, 1) {
		return nil
	}
	defer atomic.StoreInt64(&r.inUpdate, 0)

	srvMessage, errSrv := r.consulRequest(r.dnsName, dnsmessage.TypeSRV)
	if errSrv != nil {
		return fmt.Errorf("error get SRV records, %w", errSrv)
	}

	var result []string

	var srvRecords []*dnsmessage.SRVResource
	hosts := map[string]dnsmessage.Name{}

	for _, answer := range srvMessage.Answers {
		srv, ok := answer.Body.(*dnsmessage.SRVResource)
		if !ok {
			return fmt.Errorf("expect *dnsmessage.SRVResource, got %T", answer.Body)
		}

		if r.getAddressFromSRV {
			// for SRV addresses like '7f000001.addr.dc1.consul.'
			hexIP, errHexDecode := hex.DecodeString(string(srv.Target.Data[:8]))
			if errHexDecode != nil {
				return fmt.Errorf("error decode hex address, %w", errHexDecode)
			}

			result = append(result, fmt.Sprintf("%d.%d.%d.%d:%d", hexIP[0], hexIP[1], hexIP[2], hexIP[3], srv.Port))
			continue
		}

		hosts[srv.Target.String()] = srv.Target
		srvRecords = append(srvRecords, srv)
	}

	if !r.getAddressFromSRV {
		addresses := map[string]string{}

		for k, v := range hosts {
			aMessage, errA := r.consulRequest(v, dnsmessage.TypeA)
			if errA != nil {
				return fmt.Errorf("error get A records, %w", errA)
			}

			for _, answer := range aMessage.Answers {
				mm, ok := answer.Body.(*dnsmessage.AResource)
				if !ok {
					return fmt.Errorf("expect *dnsmessage.AResource, got %T", answer.Body)
				}
				addresses[k] = fmt.Sprintf("%d.%d.%d.%d", mm.A[0], mm.A[1], mm.A[2], mm.A[3])
			}
		}

		for _, srv := range srvRecords {
			ip, ok := addresses[srv.Target.String()]
			if !ok {
				return fmt.Errorf("unexpected not found info about host %s", srv.Target.String())
			}
			result = append(result, ip+":"+strconv.Itoa(int(srv.Port)))
		}
	}

	r.mx.Lock()
	r.data = r.data[:0]
	r.data = append(r.data, result...)
	r.mx.Unlock()

	return nil
}

func (r *ConsulResolver) releaseConn(conn net.Conn) {
	r.connsPool.Put(conn)
}

func (r *ConsulResolver) acquireConn() (net.Conn, error) {
	c := r.connsPool.Get()
	if c != nil {
		return c.(net.Conn), nil
	}

	d := net.Dialer{Timeout: r.timeout}
	cc, err := d.Dial("tcp", r.address)
	if err != nil {
		return nil, err
	}
	return cc, nil
}

func (r *ConsulResolver) consulRequest(name dnsmessage.Name, t dnsmessage.Type) (*dnsmessage.Message, error) {
	q := dnsmessage.Question{
		Name:  name,
		Type:  t,
		Class: dnsmessage.ClassINET,
	}
	b := dnsmessage.NewBuilder(make([]byte, 2, 514), dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, fmt.Errorf("error build message, start questions, %w", err)
	}
	if err := b.Question(q); err != nil {
		return nil, fmt.Errorf("error build message, add question, %w", err)
	}
	req, err := b.Finish()
	if err != nil {
		return nil, fmt.Errorf("error build message, finish, %w", err)
	}

	l := len(req) - 2
	req[0] = byte(l >> 8)
	req[1] = byte(l)

	for i := 0; i < r.requestAttempts; i++ {
		conn, errGetConnection := r.acquireConn()
		if errGetConnection != nil {
			return nil, fmt.Errorf("error get connection, %w", errGetConnection)
		}

		errWriteDeadline := conn.SetWriteDeadline(time.Now().Add(r.timeout))
		if errWriteDeadline != nil {
			r.logger.Printf("error set write deadline, %v", errWriteDeadline)
			errClose := conn.Close()
			if errClose != nil {
				r.logger.Printf("error close connection, %v", errClose)
			}
			continue
		}
		_, errWrite := conn.Write(req)
		if errWrite != nil {
			r.logger.Printf("error write to connection, %v", errWrite)
			errClose := conn.Close()
			if errClose != nil {
				r.logger.Printf("error close connection, %v", errClose)
			}
			continue
		}

		res := make([]byte, 0)
		buf := make([]byte, 1024)
		for {
			errReadDeadline := conn.SetDeadline(time.Now().Add(r.timeout))
			if errReadDeadline != nil {
				r.logger.Printf("error set read deadline, %v", errReadDeadline)
				errClose := conn.Close()
				if errClose != nil {
					r.logger.Printf("error close connection, %v", errClose)
				}
				break
			}
			n, errRead := conn.Read(buf)
			if errRead != nil {
				r.logger.Printf("error read from connection, %v", errRead)
				errClose := conn.Close()
				if errClose != nil {
					r.logger.Printf("error close connection, %v", errClose)
				}
				break
			}

			res = append(res, buf[:n]...)
			if n < len(buf) {
				m := &dnsmessage.Message{}
				errUnpack := m.Unpack(res[2:])
				if errUnpack != nil {
					return nil, fmt.Errorf("error unpack reponse, %w", errUnpack)
				}
				r.releaseConn(conn)
				return m, nil
			}
		}
	}

	return nil, fmt.Errorf("max attempts reached")
}
