package pongo2

import (
	"encoding/json"

	p "github.com/flosch/pongo2"
)

func ViaJson(v interface{}) (p.Context, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return p.Context{}, err
	}

	ptx := p.Context{}
	err = json.Unmarshal(data, &ptx)
	if err != nil {
		return p.Context{}, err
	}
	return ptx, nil
}

func YAMLSafeContext(v interface{}) (*p.Context, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	before := make(map[string]interface{})
	err = json.Unmarshal(data, &before)
	if err != nil {
		return nil, err
	}

	after := make(map[string]interface{})
	for k, v := range before {
		switch u := v.(type) {
		case bool:
			if u {
				after[k] = "true"
			} else {
				after[k] = "false"
			}
		default:
			after[k] = v
		}
	}

	ptx := p.Context(after)
	return &ptx, nil
}
