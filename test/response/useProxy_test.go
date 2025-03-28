package main

import (
	"testing"

	"github.com/rluisr/requests"
)

func TestUseProxy(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Error(err)
	}

	if resp.Proxys() != nil {
		t.Error("proxy error")
	}
}
