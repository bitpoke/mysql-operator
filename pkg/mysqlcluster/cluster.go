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

const (
	statusUpToDate = "up-to-date"
	statusCreated  = "created"
	statusUpdated  = "updated"
	statusFaild    = "faild"
	statusOk       = "ok"
	statusSkip     = "skip"
)

func (f *cFactory) Sync(ctx context.Context) error {
	if len(f.cl.Spec.SecretName) == 0 {
		err := fmt.Errorf("the Spec.SecretName is empty")
		f.updateState(api.ClusterConditionConfig, statusFaild, err)
		return err
	}

	s, err := f.syncDbCredentialsSecret()
	f.updateState(api.ClusterConditionConfig, s, err)
	if err != nil {
		return fmt.Errorf("db-credentials failed: %s", err)
	}
	s, err = f.syncUtilitySecret()
	f.updateState(api.ClusterConditionConfig, s, err)
	if err != nil {
		return fmt.Errorf("utility-secret failed: %s", err)
	}
	s, err = f.syncConfigEnvSecret()
	f.updateState(api.ClusterConditionConfig, s, err)
	if err != nil {
		return fmt.Errorf("config secert failed: %s", err)
	}
	s, err = f.syncConfigMapFiles()
	f.updateState(api.ClusterConditionConfig, s, err)
	if err != nil {
		return fmt.Errorf("config map failed: %s", err)
	}
	s, err = f.syncHeadlessService()
	f.updateState(api.ClusterConditionReady, s, err)
	if err != nil {
		return fmt.Errorf("headless service failed: %s", err)
	}
	s, err = f.syncStatefulSet()
	f.updateState(api.ClusterConditionReady, s, err)
	if err != nil {
		return fmt.Errorf("statefulset failed: %s", err)
	}
	return nil
}

func (f *cFactory) updateState(condType api.ClusterConditionType,
	s string, err error) {
	switch s {
	case statusCreated, statusUpdated, statusUpToDate, statusOk:
		f.cl.UpdateStatusCondition(condType, apiv1.ConditionTrue, "sync", "")
	case statusFaild:
		f.cl.UpdateStatusCondition(condType, apiv1.ConditionFalse, "sync", err.Error())
	default:
		fmt.Println("herherhehrehr")
	}
}

func (f *cFactory) syncHeadlessService() (state string, err error) {
	expHL := f.createHeadlessService()
	state = statusUpToDate
	defer func() {
		glog.V(2).Infof("headless service...(%s)", state)
	}()

	servicesClient := f.client.CoreV1().Services(f.namespace)
	hlSVC, err := servicesClient.Get(f.getNameForResource(HeadlessSVC), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = servicesClient.Create(&expHL)
		return
	} else if err != nil {
		return
	}
	if !metav1.IsControlledBy(hlSVC, f.cl) {
		// log a warning to evant recorder...
		state = statusFaild
		err = fmt.Errorf("The service is not controlled by this resource!")
		return
	}

	// TODO: find a better condition
	if !reflect.DeepEqual(hlSVC.Spec.Ports, expHL.Spec.Ports) {
		state = statusUpdated
		expHL.SetResourceVersion(hlSVC.GetResourceVersion())
		_, err = servicesClient.Update(&expHL)
		return
	}

	return
}

func (f *cFactory) syncConfigEnvSecret() (state string, err error) {
	expCS := f.createEnvConfigSecret()
	state = statusUpToDate
	defer func() {
		glog.V(2).Infof("secret config env...(%s)", state)
	}()

	scrtClient := f.client.CoreV1().Secrets(f.namespace)
	cfgSct, err := scrtClient.Get(f.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = scrtClient.Create(&expCS)
		return
	} else if err != nil {
		return
	}
	if !metav1.IsControlledBy(cfgSct, f.cl) {
		// log a warning to evant recorder...
		err = fmt.Errorf("The config env map is not controlled by this resource!")
		return
	}

	if !reflect.DeepEqual(cfgSct.Data, expCS.Data) {
		state = statusUpdated
		expCS.SetResourceVersion(cfgSct.GetResourceVersion())
		_, err = scrtClient.Update(&expCS)
		return
	}

	return
}

func (f *cFactory) syncConfigMapFiles() (state string, err error) {
	expCM := f.createConfigMapFiles()
	state = statusUpToDate
	defer func() {
		glog.V(2).Infof("config map...(%s)", state)
	}()

	cmClient := f.client.CoreV1().ConfigMaps(f.namespace)
	cfgMap, err := cmClient.Get(f.getNameForResource(ConfigMap), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = cmClient.Create(&expCM)
		return
	} else if err != nil {
		return
	}

	if !metav1.IsControlledBy(cfgMap, f.cl) {
		state = statusSkip
		// log a warning to evant recorder...
		err = fmt.Errorf("The config map files is not controlled by this resource!")
		return
	}

	if !reflect.DeepEqual(cfgMap.Data, expCM.Data) {
		state = statusUpdated
		expCM.SetResourceVersion(cfgMap.GetResourceVersion())
		_, err = cmClient.Update(&expCM)
		return
	}

	return
}

func (f *cFactory) syncDbCredentialsSecret() (state string, err error) {
	state = statusOk
	defer func() {
		glog.V(2).Infof("db credentials...(%s)", state)
	}()
	expSec := f.createDbCredentialSecret(f.cl.Spec.SecretName)
	if _, err = kube.EnsureSecretKeys(f.client, expSec, true); err != nil {
		state = statusFaild
		err = fmt.Errorf("fail to ensure secret: %s", err)
		return
	}

	return
}

func (f *cFactory) syncStatefulSet() (state string, err error) {
	expSS := f.createStatefulSet()
	state = statusUpToDate
	defer func() {
		glog.V(2).Infof("statefulset...(%s)", state)
	}()

	sfsClient := f.client.AppsV1beta2().StatefulSets(f.namespace)
	sfs, err := sfsClient.Get(f.getNameForResource(StatefulSet), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = sfsClient.Create(&expSS)
		return
	} else if err != nil {
		state = statusFaild
		return
	}

	if !metav1.IsControlledBy(sfs, f.cl) {
		// log a warning to evant recorder...
		state = statusFaild
		err = fmt.Errorf("The config map files is not controlled by this resource!")
		return
	}

	if !statefulSetEqual(sfs, &expSS) {
		state = statusUpdated
		expSS.SetResourceVersion(sfs.GetResourceVersion())
		_, err = sfsClient.Update(&expSS)
		return
	}

	if !reflect.DeepEqual(sfs.Spec, expSS.Spec) {
		state = statusSkip
		glog.V(2).Infof("statefulSet has changes that can't be applied")
	}

	return
}

func (f *cFactory) syncUtilitySecret() (state string, err error) {
	expS := f.createUtilitySecret()
	state = statusOk
	defer func() {
		glog.V(2).Infof("utility secret...(%s)", state)
	}()

	if _, err = kube.EnsureSecretKeys(f.client, expS, true); err != nil {
		state = statusFaild
		err = fmt.Errorf("fail to ensure secret: %s", err)
		return
	}
	return

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
// TODO: remove if not used
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
	return true
}
