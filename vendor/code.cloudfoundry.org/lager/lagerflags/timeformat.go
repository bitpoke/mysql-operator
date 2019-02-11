package lagerflags

import (
	"errors"
	"fmt"
)

type TimeFormat int

const (
	FormatUnixEpoch TimeFormat = iota
	FormatRFC3339
)

func (t TimeFormat) MarshalJSON() ([]byte, error) {
	if FormatUnixEpoch <= t && t <= FormatRFC3339 {
		return []byte(`"` + t.String() + `"`), nil
	}
	return nil, fmt.Errorf("invalid TimeFormat: %d", t)
}

// Set implements the flag.Getter interface
func (t TimeFormat) Get(s string) interface{} { return t }

// Set implements the flag.Value interface
func (t *TimeFormat) Set(s string) error {
	switch s {
	case "unix-epoch", "0":
		*t = FormatUnixEpoch
	case "rfc3339", "1":
		*t = FormatRFC3339
	default:
		return errors.New(`invalid TimeFormat: "` + s + `"`)
	}
	return nil
}

func (t *TimeFormat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	// unqote
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	return t.Set(string(data))
}

func (t TimeFormat) String() string {
	switch t {
	case FormatUnixEpoch:
		return "unix-epoch"
	case FormatRFC3339:
		return "rfc3339"
	}
	return "invalid"
}
