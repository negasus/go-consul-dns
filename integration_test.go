package go_consul_dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestIntegration_inUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration")
		return
	}

	r, errNew := New("bar", WithConsulAddress("127.0.0.1:18600"))
	if errNew != nil {
		t.Error(errNew)
		return
	}
	defer r.Close()

	r.inUpdate = 1

	errUpdate := r.Update()
	if errUpdate != nil {
		t.Error(errUpdate)
		return
	}

	if len(r.data) != 0 {
		t.Errorf("unexpcted data len")
	}
}

func TestIntegration_notExistsService(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration")
		return
	}

	r, errNew := New("notexists", WithConsulAddress("127.0.0.1:18600"))
	if errNew != nil {
		t.Error(errNew)
		return
	}
	defer r.Close()

	errUpdate := r.Update()
	if errUpdate != nil {
		t.Error(errUpdate)
		return
	}

	all := r.All()
	if len(all) != 0 {
		t.Errorf("unexpected seriveces count %d, expect 0", len(all))
		return
	}
}

func TestIntegrationA(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration")
		return
	}

	count := 300

	errReg := registerServices("foo", []string{"127.0.0.1", "192.168.1.42", "10.20.30.40"}, 100)
	if errReg != nil {
		t.Errorf("error register services, %v", errReg)
		return
	}

	r, errNew := New("foo", WithConsulAddress("127.0.0.1:18600"))
	if errNew != nil {
		t.Error(errNew)
		return
	}
	defer r.Close()

	errUpdate := r.Update()
	if errUpdate != nil {
		t.Error(errUpdate)
		return
	}

	all := r.All()
	if len(all) != count {
		t.Errorf("unexpected seriveces count %d, expect %d", len(all), count)
		return
	}

	addresses := map[string]struct{}{}

	for i := 0; i < count; i++ {
		if _, ok := addresses[all[i]]; ok {
			t.Error("address already exists")
			return
		}
		addresses[all[i]] = struct{}{}
	}
}

func TestIntegrationSRV(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration")
		return
	}

	count := 300

	errReg := registerServices("foo", []string{"127.0.0.1", "192.168.1.42", "10.20.30.40"}, 100)
	if errReg != nil {
		t.Errorf("error register services, %v", errReg)
		return
	}

	r, errNew := New("foo", WithConsulAddress("127.0.0.1:18600"), WithGetAddressFromSRV())
	if errNew != nil {
		t.Error(errNew)
		return
	}
	defer r.Close()

	errUpdate := r.Update()
	if errUpdate != nil {
		t.Error(errUpdate)
		return
	}

	all := r.All()
	if len(all) != count {
		t.Errorf("unexpected seriveces count %d, expect %d", len(all), count)
		return
	}

	addresses := map[string]struct{}{}

	for i := 0; i < count; i++ {
		if _, ok := addresses[all[i]]; ok {
			t.Error("address already exists")
			return
		}
		addresses[all[i]] = struct{}{}
	}
}

type consulRequest struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

func registerServices(name string, addresses []string, count int) error {
	var id int
	for _, address := range addresses {
		startPort := 2000
		for i := 0; i < count; i++ {
			id++
			startPort++

			payload := consulRequest{
				ID:      fmt.Sprintf("%s_%d", name, id),
				Name:    name,
				Address: address,
				Port:    startPort,
			}

			body, errMarshal := json.Marshal(&payload)
			if errMarshal != nil {
				return fmt.Errorf("error marshal payload, %w", errMarshal)
			}

			req, errReq := http.NewRequest(http.MethodPut, "http://127.0.0.1:18500/v1/agent/service/register", bytes.NewReader(body))
			if errReq != nil {
				return errReq
			}
			req.Header.Add("content-type", "application/json")

			resp, errDo := http.DefaultClient.Do(req)
			if errDo != nil {
				return fmt.Errorf("error do request %d, %w", i, errDo)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
		}
	}
	return nil
}
