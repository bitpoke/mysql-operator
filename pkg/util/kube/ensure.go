package kube

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EnsureSecretKeys ensures that the provided secret exists and has the
// specified keys. If the secret exists, it's values are updated. The values are
// overwrited unless keep parameter is false.
func EnsureSecretKeys(cl kubernetes.Interface,
	secret *apiv1.Secret, keep bool,
) (*apiv1.Secret, error) {
	s, err := cl.Core().Secrets(secret.Namespace).Create(secret)
	if err != nil && errors.IsAlreadyExists(err) {
		s, err := cl.Core().Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{ResourceVersion: "0"})
		if err != nil {
			return nil, err
		}
		for key, value := range secret.Data {
			_, exists := s.Data[key]
			if keep && exists {
				continue
			}
			s.Data[key] = value
		}
		return cl.Core().Secrets(secret.Namespace).Update(s)
	}
	return s, err

}
