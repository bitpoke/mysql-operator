package types

import (
	"encoding/json"
)

type JsonMap map[string]interface{}

func (j *JsonMap) FromDB(bytes []byte) error {
	*j = make(map[string]interface{})
	return json.Unmarshal(bytes, j)
}

func (j *JsonMap) ToDB() ([]byte, error) {
	if len(*j) == 0 {
		return j.Default(), nil
	}
	return json.Marshal(j)
}

func (j *JsonMap) Default() []byte {
	return []byte("{}")
}

func (j *JsonMap) String() string {
	bytes, err := j.ToDB()
	if err != nil {
		return string(j.Default())
	}
	return string(bytes)
}
