package net

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestIPInfo(t *testing.T) {
	if resp, err := http.Get("https://ipinfo.io/ip"); err == nil {
		defer resp.Body.Close()
		if bytes, err := ioutil.ReadAll(resp.Body); err == nil {
			fmt.Println(string(bytes))
			ip := net.ParseIP(strings.TrimSpace(string(bytes)))
			fmt.Println(ip)
			if ip != nil {
				ip = ip.To4()
			}
			if ip != nil {
				fmt.Println(ip)
			}
		}
	}
}
