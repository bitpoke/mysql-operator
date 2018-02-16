package v1

import (
	"errors"

	"github.com/appscode/kutil/meta"
	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetGroupVersionKind(v interface{}) schema.GroupVersionKind {
	return batch.SchemeGroupVersion.WithKind(meta.GetKind(v))
}

func AssignTypeKind(v interface{}) error {
	_, err := conversion.EnforcePtr(v)
	if err != nil {
		return err
	}

	switch u := v.(type) {
	case *batch.Job:
		u.APIVersion = batch.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	}
	return errors.New("unknown api object type")
}
