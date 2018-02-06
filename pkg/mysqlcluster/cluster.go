package mysqlcluster

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	"github.com/presslabs/titanium/pkg/util/kube"
	"github.com/presslabs/titanium/pkg/util/options"
)

type Interface interface {
	Sync(ctx context.Context) error
}

// cluster factory
type cFactory struct {
	cl  *api.MysqlCluster
	opt options.Options

	namespace string

	client   kubernetes.Interface
	cmclient clientset.Interface

	hlService      apiv1.Service
	statefulSet    v1beta2.StatefulSet
	configSecret   apiv1.Secret
	configMapFiles apiv1.ConfigMap
}

func New(cl *api.MysqlCluster, klient kubernetes.Interface,
	cmclient clientset.Interface, ns string) Interface {
	return &cFactory{
		cl:        cl,
		client:    klient,
		cmclient:  cmclient,
		namespace: ns,
	}
}

func (f *cFactory) Sync(ctx context.Context) error {
	if len(f.cl.Spec.SecretName) == 0 {
		return fmt.Errorf("the Spec.SecretName is empty")
	}
	if err := f.syncDbCredentialsSecret(); err != nil {
		return fmt.Errorf("db-credentials failed: %s", err)
	}
	if err := f.syncHeadlessService(); err != nil {
		return fmt.Errorf("headless service failed: %s", err)
	}
	if err := f.syncConfigEnvSecret(); err != nil {
		return fmt.Errorf("config secert failed: %s", err)
	}
	if err := f.syncConfigMapFiles(); err != nil {
		return fmt.Errorf("config map failed: %s", err)
	}
	if err := f.syncStatefulSet(); err != nil {
		return fmt.Errorf("statefulset failed: %s", err)
	}
	return nil
}

func (f *cFactory) syncHeadlessService() error {
	expHL := f.createHeadlessService()
	state := "up-to-date"
	defer f.infof(3, "headless service...(%s)", state)

	servicesClient := f.client.CoreV1().Services(f.namespace)
	hlSVC, err := servicesClient.Get(f.getNameForResource(HeadlessSVC), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = "created"
		_, err = servicesClient.Create(&expHL)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(hlSVC, f.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The service is not controlled by this resource!")
	}

	// TODO: find a better condition
	if !reflect.DeepEqual(hlSVC.Spec.Ports, expHL.Spec.Ports) {
		state = "updated"
		expHL.SetResourceVersion(hlSVC.GetResourceVersion())
		_, err := servicesClient.Update(&expHL)
		return err
	}

	return nil
}

func (f *cFactory) syncConfigEnvSecret() error {
	expCS := f.createEnvConfigSecret()

	scrtClient := f.client.CoreV1().Secrets(f.namespace)
	cfgSct, err := scrtClient.Get(f.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Println("ConfSecret ... created")
		_, err = scrtClient.Create(&expCS)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(cfgSct, f.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config env map is not controlled by this resource!")
	}

	if !reflect.DeepEqual(cfgSct.Data, expCS.Data) {
		fmt.Println("ConfSecret ... updated")
		expCS.SetResourceVersion(cfgSct.GetResourceVersion())
		_, err := scrtClient.Update(&expCS)
		return err
	}

	fmt.Println("ConfSecret ... up-to-date")
	return nil
}

func (f *cFactory) syncConfigMapFiles() error {
	expCM := f.createConfigMapFiles()
	cmClient := f.client.CoreV1().ConfigMaps(f.namespace)
	cfgMap, err := cmClient.Get(f.getNameForResource(ConfigMap), metav1.GetOptions{})

	if errors.IsNotFound(err) {
		fmt.Println("ConfigMap ... created")
		_, err = cmClient.Create(&expCM)
		return err
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(cfgMap, f.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config map files is not controlled by this resource!")
	}

	if !reflect.DeepEqual(cfgMap.Data, expCM.Data) {
		fmt.Println("ConfigMap ... updated")
		expCM.SetResourceVersion(cfgMap.GetResourceVersion())
		_, err := cmClient.Update(&expCM)
		return err
	}

	fmt.Println("ConfigMap ... up-to-date")
	return nil
}

func (f *cFactory) syncDbCredentialsSecret() error {
	expSec := f.createDbCredentialSecret(f.cl.Spec.SecretName)
	if _, err := kube.EnsureSecretKeys(f.client, expSec, true); err != nil {
		return fmt.Errorf("fail to ensure secret: %s", err)
	}

	fmt.Println("db-credentials ... ok")
	return nil
}

func (f *cFactory) syncStatefulSet() error {
	expSS := f.createStatefulSet()

	sfsClient := f.client.AppsV1beta2().StatefulSets(f.namespace)
	sfs, err := sfsClient.Get(f.getNameForResource(StatefulSet), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Println("StatefulSet ... created")
		_, err = sfsClient.Create(&expSS)
		return err
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(sfs, f.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config map files is not controlled by this resource!")
	}

	if !statefulSetEqual(sfs, &expSS) {
		fmt.Println("StatefulSet ... updated")
		expSS.SetResourceVersion(sfs.GetResourceVersion())
		_, err = sfsClient.Update(&expSS)
		return err
	}

	if !reflect.DeepEqual(sfs.Spec, expSS.Spec) {
		fmt.Println("StatefulSet has changes that can't be applied.")
	}

	fmt.Println("StatefulSet ... up-to-date")
	return nil
}

func (f *cFactory) getOwnerReferences(ors ...[]metav1.OwnerReference) []metav1.OwnerReference {
	rs := []metav1.OwnerReference{
		f.cl.AsOwnerReference(),
	}
	for _, or := range ors {
		for _, o := range or {
			rs = append(rs, o)
		}
	}
	return rs
}

// This method checks if given secret exists in cluster namespace.
func (f *cFactory) existsSecret(name string) bool {
	if len(name) == 0 {
		return false
	}

	client := f.client.CoreV1().Secrets(f.namespace)
	if _, err := client.Get(name, metav1.GetOptions{}); err != nil {
		return false
	}

	return true
}

func statefulSetEqual(a, b *v1beta2.StatefulSet) bool {
	if *a.Spec.Replicas != *b.Spec.Replicas {
		return false
	}

	//if !reflect.DeepEqual(a.Spec.Template, b.Spec.Template) {
	//	return false
	//}

	//if !reflect.DeepEqual(a.Spec.UpdateStrategy, b.Spec.UpdateStrategy) {
	//	return false
	//}
	return true
}

func (f *cFactory) infof(lv glog.Level, fmtS string, args ...interface{}) {
	glog.V(lv).Infof(fmtS, args...)
}
