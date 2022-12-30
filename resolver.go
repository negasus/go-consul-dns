package go_consul_dns

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var (
	defaultTimeout       = time.Second * 10
	defaultConsulAddress = "127.0.0.1:8600"
	defaultDatacenter    = "dc1"
	defaultDomain        = "consul"
)

type Resolver interface {
	Update() error
	All() []string
	Random() string
	Next() string
	Close() error
}

type ConsulResolver struct {
	service string

	address    string
	datacenter string
	domain     string
	timeout    time.Duration

	mx       *sync.RWMutex
	data     []string
	conn     net.Conn
	inUpdate int64

	counter int64
}

func New(service string, opts ...Option) (*ConsulResolver, error) {
	r := &ConsulResolver{
		address:    defaultConsulAddress,
		datacenter: defaultDatacenter,
		domain:     defaultDomain,
		timeout:    defaultTimeout,
		mx:         &sync.RWMutex{},
	}

	for _, o := range opts {
		o(r)
	}

	r.service = service + ".service." + r.datacenter + "." + r.domain + "."

	var errDial error
	r.conn, errDial = net.Dial("tcp", r.address)
	if errDial != nil {
		return nil, fmt.Errorf("error connect to consul %q, %w", r.address, errDial)
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

func (r *ConsulResolver) Close() error {
	return r.conn.Close()
}

func (r *ConsulResolver) Update() error {
	if !atomic.CompareAndSwapInt64(&r.inUpdate, 0, 1) {
		return nil
	}
	defer atomic.StoreInt64(&r.inUpdate, 0)

	req, errReq := r.buildRequest()
	if errReq != nil {
		return fmt.Errorf("error build request, %w", errReq)
	}

	errWriteDeadline := r.conn.SetWriteDeadline(time.Now().Add(r.timeout))
	if errWriteDeadline != nil {
		return fmt.Errorf("error set write deadline, %w", errWriteDeadline)
	}
	_, errWrite := r.conn.Write(req)
	if errWrite != nil {
		return fmt.Errorf("error write data to connection, %w", errWrite)
	}

	res := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		errReadDeadline := r.conn.SetDeadline(time.Now().Add(r.timeout))
		if errReadDeadline != nil {
			return fmt.Errorf("error set read deadline, %w", errReadDeadline)
		}
		n, errRead := r.conn.Read(buf)
		if errRead != nil {
			return fmt.Errorf("error read data from connection, %w", errRead)
		}

		res = append(res, buf[:n]...)
		if n < len(buf) {
			break
		}
	}

	m := &dnsmessage.Message{}
	errUnpack := m.Unpack(res[2:])
	if errUnpack != nil {
		return fmt.Errorf("error unpack reponse, %w", errUnpack)
	}

	var result []string

	for _, answer := range m.Answers {
		srv, ok := answer.Body.(*dnsmessage.SRVResource)
		if !ok {
			return fmt.Errorf("answer is not SRV resource")
		}
		hexIP, errHexDecode := hex.DecodeString(string(srv.Target.Data[:8]))
		if errHexDecode != nil {
			return fmt.Errorf("error decode hex address, %w", errHexDecode)
		}

		result = append(result, fmt.Sprintf("%d.%d.%d.%d:%d", hexIP[0], hexIP[1], hexIP[2], hexIP[3], srv.Port))
	}

	r.mx.Lock()
	r.data = r.data[:0]
	r.data = append(r.data, result...)
	r.mx.Unlock()
	return nil
}

func (r *ConsulResolver) buildRequest() ([]byte, error) {
	n, errName := dnsmessage.NewName(r.service)
	if errName != nil {
		return nil, fmt.Errorf("error parse name, %w", errName)
	}
	q := dnsmessage.Question{
		Name:  n,
		Type:  dnsmessage.TypeSRV,
		Class: dnsmessage.ClassINET,
	}

	b := dnsmessage.NewBuilder(make([]byte, 2, 514), dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, fmt.Errorf("error start questions, %w", err)
	}
	if err := b.Question(q); err != nil {
		return nil, fmt.Errorf("error add question, %w", err)
	}
	req, err := b.Finish()
	if err != nil {
		return nil, fmt.Errorf("error finish, %w", err)
	}

	l := len(req) - 2
	req[0] = byte(l >> 8)
	req[1] = byte(l)

	return req, nil
}
