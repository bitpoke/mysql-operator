package ntp

import (
	"fmt"
	"time"

	"github.com/beevik/ntp"
)

func CheckSkew(limit time.Duration) error {
	t, err := ntp.Time("0.pool.ntp.org")
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
