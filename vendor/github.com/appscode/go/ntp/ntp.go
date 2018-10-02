package ntp

import (
	"fmt"
	"time"

	"github.com/beevik/ntp"
)

func CheckSkew(limit time.Duration) error {
	return CheckSkewFromServer("0.pool.ntp.org", limit)
}

func CheckSkewFromServer(server string, limit time.Duration) error {
	t, err := ntp.Time(server)
	if err != nil {
		return err
	}
	n := time.Now()
	d := n.Sub(t)
	if n.Before(t) {
		d = t.Sub(n)
	}
	if d > limit {
		return fmt.Errorf("Time skew between NTP(%v) & machine(%v) exceedes limit", t, n)
	}
	return nil
}
