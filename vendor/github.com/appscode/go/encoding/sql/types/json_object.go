package types

import (
	"encoding/json"
)

type JsonObj []byte

func (j *JsonObj) FromDB(bytes []byte) error {
	*j = JsonObj(bytes)
	return nil
}

func (j *JsonObj) ToDB() ([]byte, error) {
	if j.IsEmpty() {
		return j.Default(), nil
	}
	return []byte(*j), nil
}

func (j *JsonObj) IsEmpty() bool {
	return len([]byte(*j)) == 0
}

func (j *JsonObj) Default() []byte {
	return []byte("{}")
}

func (j *JsonObj) Marshal(v interface{}) (err error) {
	*j, err = json.Marshal(v)
	return
}

func (j *JsonObj) Unmarshal(v interface{}) error {
	return json.Unmarshal([]byte(*j), v)
}

func (j *JsonObj) String() string {
	return string([]byte(*j))
}

func (j *JsonObj) Bytes() []byte {
	return []byte(*j)
}

func NewJsonObj(v interface{}) (JsonObj, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return JsonObj(b), nil
}
