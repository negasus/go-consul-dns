package go_consul_dns

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew_Error_NoConnection(t *testing.T) {
	_, err := New("foo", WithConsulAddress("bad_address"))
	if err == nil {
		t.Fatal("unexpected no error")
	}

	if err.Error() != "error connect to consul \"bad_address\", dial tcp: address bad_address: missing port in address" {
		t.Fatalf("unexpected error message %s", err.Error())
	}
}

func TestTimeout(t *testing.T) {
	ln, errLn := net.Listen("tcp", "127.0.0.1:38600")
	if errLn != nil {
		t.Fatalf("error listen address, %v", errLn)
	}
	defer ln.Close()

	r, err := New("foo", WithConsulAddress("127.0.0.1:38600"), WithTimeout(time.Millisecond*100))
	if err != nil {
		t.Errorf("unexpected error, %v", err)
		return
	}
	defer r.Close()

	errUpdate := r.Update()
	if errUpdate == nil {
		t.Error("unexpected error is nil")
		return
	}

	if !strings.HasSuffix(errUpdate.Error(), "i/o timeout") {
		t.Errorf("unexpected error message, %v", errUpdate)
		return
	}
}

func TestNew_Options(t *testing.T) {
	opts := []Option{
		WithConsulAddress("127.0.0.1:38600"),
		WithDatacenter("dc10"),
		WithDomain("domain"),
		WithTimeout(time.Second),
	}

	ln, errLn := net.Listen("tcp", "127.0.0.1:38600")
	if errLn != nil {
		t.Fatalf("error listen address, %v", errLn)
	}
	defer ln.Close()

	r, err := New("foo", opts...)
	if err != nil {
		t.Errorf("unexpected error, %v", err)
		return
	}
	defer r.Close()

	if r.address != "127.0.0.1:38600" {
		t.Errorf("unexpected address value %s", r.address)
		return
	}
	if r.datacenter != "dc10" {
		t.Errorf("unexpected datacenter value %s", r.address)
		return
	}
	if r.domain != "domain" {
		t.Errorf("unexpected domain value %s", r.address)
		return
	}
	if r.timeout != time.Second {
		t.Errorf("unexpected timeout value %s", r.address)
		return
	}
}

func TestConsulResolver_Random_Empty(t *testing.T) {
	r := &ConsulResolver{
		mx: &sync.RWMutex{},
	}

	v := r.Random()
	if v != "" {
		t.Fatalf("unexpected result %s", v)
	}
}

func TestConsulResolver_Next_Empty(t *testing.T) {
	r := &ConsulResolver{
		mx: &sync.RWMutex{},
	}

	v := r.Next()
	if v != "" {
		t.Fatalf("unexpected result %s", v)
	}
}

func TestConsulResolver_Random(t *testing.T) {
	r := &ConsulResolver{
		mx:   &sync.RWMutex{},
		data: []string{"one", "two", "three"},
	}

	var v string

	res := map[string]int{}

	for i := 0; i < 100; i++ {
		v = r.Random()
		res[v]++
	}

	if len(res) != 3 {
		t.Fatalf("unexpected results len %d", len(res))
	}
	var ok bool

	_, ok = res["one"]
	if !ok {
		t.Fatal("not ok")
	}
	_, ok = res["two"]
	if !ok {
		t.Fatal("not ok")
	}
	_, ok = res["three"]
	if !ok {
		t.Fatal("not ok")
	}
}

func TestConsulResolver_Next(t *testing.T) {
	r := &ConsulResolver{
		mx:   &sync.RWMutex{},
		data: []string{"one", "two", "three"},
	}

	var v string

	v = r.Next()
	if v != "one" {
		t.Fatal("unexpected")
	}
	v = r.Next()
	if v != "two" {
		t.Fatal("unexpected")
	}
	v = r.Next()
	if v != "three" {
		t.Fatal("unexpected")
	}
	v = r.Next()
	if v != "one" {
		t.Fatal("unexpected")
	}
	v = r.Next()
	if v != "two" {
		t.Fatal("unexpected")
	}
}

func TestConsulResolver_All(t *testing.T) {
	r := &ConsulResolver{
		mx:   &sync.RWMutex{},
		data: []string{"one", "two", "three"},
	}

	v := r.All()
	if len(v) != 3 {
		t.Fatal("wrong count")
	}
	if v[0] != "one" && v[1] != "two" && v[2] != "three" {
		t.Fatal("unexpected output")
	}
}
