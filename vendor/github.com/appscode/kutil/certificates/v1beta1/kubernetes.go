package v1beta1

import (
	"errors"

	"github.com/appscode/kutil/meta"
	certificates "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetGroupVersionKind(v interface{}) schema.GroupVersionKind {
	return certificates.SchemeGroupVersion.WithKind(meta.GetKind(v))
}

func AssignTypeKind(v interface{}) error {
	_, err := conversion.EnforcePtr(v)
	if err != nil {
		return err
	}

	switch u := v.(type) {
	case *certificates.CertificateSigningRequest:
		u.APIVersion = certificates.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	}
	return errors.New("unknown api object type")
}
