package types

import (
	"encoding/json"
)

type JsonArr []byte

func (j *JsonArr) FromDB(bytes []byte) error {
	*j = JsonArr(bytes)
	return nil
}

func (j *JsonArr) ToDB() ([]byte, error) {
	if j.IsEmpty() {
		return j.Default(), nil
	}
	return []byte(*j), nil
}

func (j *JsonArr) IsEmpty() bool {
	return len([]byte(*j)) == 0
}

func (j *JsonArr) Default() []byte {
	return []byte("[]")
}

func (j *JsonArr) Marshal(v interface{}) (err error) {
	*j, err = json.Marshal(v)
	return
}

func (j *JsonArr) Unmarshal(v interface{}) error {
	return json.Unmarshal([]byte(*j), v)
}

func (j *JsonArr) String() string {
	return string([]byte(*j))
}

func (j *JsonArr) Bytes() []byte {
	return []byte(*j)
}

func NewJsonArr(v interface{}) (JsonArr, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return JsonArr(b), nil
}
