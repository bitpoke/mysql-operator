package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func GetHostname() string {
	return os.Getenv("HOSTNAME")
}

func NodeRole() string {
	if strings.Contains(GetHostname(), "master") {
		return "master"
	} else {
		return "slave"
	}
}

func getOrdinal() int {
	hn := GetHostname()
	// mysql-master-1
	// or
	// stateful-ceva-3
	l := strings.Split(hn, "-")
	for i := len(l) - 1; i >= 0; i-- {
		if o, err := strconv.ParseInt(l[i], 10, 8); err == nil {
			return int(o)
		}
	}

	return 0
}

func GetServerId() int {
	o := getOrdinal()

	var id int
	if NodeRole() == "master" {
		id = o
	} else {
		id = o + 100
	}

	return id
}

// GetHostFor returns the host for given server id
func GetHostFor(id int) string {
	base := strings.Split(GetHostname(), "-")
	govSVC := os.Getenv("TITANIUM_HEADLESS_SERVICE")
	return fmt.Sprintf("%s-%d.%s", base, id, govSVC)
}
