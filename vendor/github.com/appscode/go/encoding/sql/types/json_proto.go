package types

import (
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

type JsonProto []byte

func (j *JsonProto) FromDB(bytes []byte) error {
	*j = JsonProto(bytes)
	return nil
}

func (j *JsonProto) ToDB() ([]byte, error) {
	if j.IsEmpty() {
		return j.Default(), nil
	}
	return []byte(*j), nil
}

func (j *JsonProto) IsEmpty() bool {
	return len([]byte(*j)) == 0
}

func (j *JsonProto) Default() []byte {
	return []byte("{}")
}

func (j *JsonProto) Marshal(pb proto.Message) (err error) {
	m := jsonpb.Marshaler{}
	s, err := m.MarshalToString(pb)
	*j = []byte(s)
	return
}

func (j *JsonProto) Unmarshal(pb proto.Message) error {
	return jsonpb.UnmarshalString(j.String(), pb)
}

func (j *JsonProto) String() string {
	return string([]byte(*j))
}

func (j *JsonProto) Bytes() []byte {
	return []byte(*j)
}

func NewJsonpb(pb proto.Message) (JsonProto, error) {
	m := jsonpb.Marshaler{}
	s, err := m.MarshalToString(pb)
	if err != nil {
		return nil, err
	}
	return JsonProto([]byte(s)), nil
}
