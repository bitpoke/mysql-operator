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
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	"github.com/presslabs/titanium/pkg/util/kube"
	"github.com/presslabs/titanium/pkg/util/options"
)

// Interface is for cluster Factory
type Interface interface {
	// Sync is the method that tries to sync the cluster.
	Sync(ctx context.Context) error
}

// cluster factory
type cFactory struct {
	cl  *api.MysqlCluster
	opt options.Options

	namespace string

	client   kubernetes.Interface
	cmclient clientset.Interface

	rec record.EventRecorder

	hlService      apiv1.Service
	statefulSet    v1beta2.StatefulSet
	configSecret   apiv1.Secret
	configMapFiles apiv1.ConfigMap
}

// New creates a new cluster factory
func New(cl *api.MysqlCluster, klient kubernetes.Interface,
	cmclient clientset.Interface, ns string, rec record.EventRecorder) Interface {
	return &cFactory{
		cl:        cl,
		client:    klient,
		cmclient:  cmclient,
		namespace: ns,
		rec:       rec,
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
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonDbSecretFaild,
			"faild to sync db-credentials secret: %s", err)
		return err
	}

	if err := f.syncDbCredentialsSecret(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonDbSecretFaild,
			"faild to sync db-credentials secret: %s", err)
		return fmt.Errorf("db-credentials failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonDbSecretUpdated,
		"db credentials secret was updated")

	if err := f.syncUtilitySecret(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonUtilitySecretFaild,
			"faild to sync utility secret: %s", err)
		return fmt.Errorf("utility-secret failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonUtilitySecretUpdated,
		"utility secret was updated")

	if err := f.syncConfigEnvSecret(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonEnvSecretFaild,
			"faild to sync config env secret: %s", err)
		return fmt.Errorf("config secert failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonEnvSecretUpdated,
		"config env secret was updated")

	if err := f.syncConfigMapFiles(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonConfigMapFaild,
			"faild to sync mysql config map: %s", err)
		return fmt.Errorf("config map failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonConfigMapUpdated,
		"config map was updated")

	if err := f.syncHeadlessService(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonServiceFaild,
			"faild to sync headless service: %s", err)
		return fmt.Errorf("headless service failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonServiceUpdated,
		"headless service was updated")

	if err := f.syncStatefulSet(); err != nil {
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonSFSFaild,
			"faild to sync statefulset: %s", err)
		return fmt.Errorf("statefulset failed: %s", err)
	}
	f.rec.Eventf(f.cl, api.EventNormal, api.EventReasonSFSUpdated,
		"statefulset was updated")
	return nil
}

func (f *cFactory) syncHeadlessService() error {
	expHL := f.createHeadlessService()
	state := statusUpToDate
	defer func() {
		glog.V(2).Infof("headless service...(%s)", state)
	}()

	servicesClient := f.client.CoreV1().Services(f.namespace)
	hlSVC, err := servicesClient.Get(f.getNameForResource(HeadlessSVC), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = servicesClient.Create(&expHL)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(hlSVC, f.cl) {
		// log a warning to evant recorder...
		state = statusFaild
		return fmt.Errorf("the selected resource not controlled by me")
	}

	// TODO: find a better condition
	if !reflect.DeepEqual(hlSVC.Spec.Ports, expHL.Spec.Ports) {
		state = statusUpdated
		expHL.SetResourceVersion(hlSVC.GetResourceVersion())
		_, err = servicesClient.Update(&expHL)
		return err
	}

	return nil
}

func (f *cFactory) syncConfigEnvSecret() error {
	expCS := f.createEnvConfigSecret()
	state := statusUpToDate
	defer func() {
		glog.V(2).Infof("secret config env...(%s)", state)
	}()

	scrtClient := f.client.CoreV1().Secrets(f.namespace)
	cfgSct, err := scrtClient.Get(f.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = scrtClient.Create(&expCS)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(cfgSct, f.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("the selected resource not controlled by me")
	}

	if !reflect.DeepEqual(cfgSct.Data, expCS.Data) {
		state = statusUpdated
		expCS.SetResourceVersion(cfgSct.GetResourceVersion())
		_, err = scrtClient.Update(&expCS)
		return err
	}

	return nil
}

func (f *cFactory) syncConfigMapFiles() error {
	expCM := f.createConfigMapFiles()
	state := statusUpToDate
	defer func() {
		glog.V(2).Infof("config map...(%s)", state)
	}()

	cmClient := f.client.CoreV1().ConfigMaps(f.namespace)
	cfgMap, err := cmClient.Get(f.getNameForResource(ConfigMap), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = cmClient.Create(&expCM)
		return err
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(cfgMap, f.cl) {
		state = statusSkip
		// log a warning to evant recorder...
		return fmt.Errorf("the selected resource not controlled by me")
	}

	if !reflect.DeepEqual(cfgMap.Data, expCM.Data) {
		state = statusUpdated
		expCM.SetResourceVersion(cfgMap.GetResourceVersion())
		_, err = cmClient.Update(&expCM)
		return err
	}

	return err
}

func (f *cFactory) syncDbCredentialsSecret() error {
	state := statusOk
	defer func() {
		glog.V(2).Infof("db credentials...(%s)", state)
	}()
	expSec := f.createDbCredentialSecret(f.cl.Spec.SecretName)
	if _, err := kube.EnsureSecretKeys(f.client, expSec, true); err != nil {
		state = statusFaild
		return fmt.Errorf("fail to ensure secret: %s", err)
	}

	return nil
}

func (f *cFactory) syncStatefulSet() error {
	expSS := f.createStatefulSet()
	state := statusUpToDate
	defer func() {
		glog.V(2).Infof("statefulset...(%s)", state)
	}()

	sfsClient := f.client.AppsV1beta2().StatefulSets(f.namespace)
	sfs, err := sfsClient.Get(f.getNameForResource(StatefulSet), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = sfsClient.Create(&expSS)
		return err
	} else if err != nil {
		state = statusFaild
		return err
	}

	if !metav1.IsControlledBy(sfs, f.cl) {
		// log a warning to evant recorder...
		state = statusFaild
		return fmt.Errorf("the selected resource not controlled by me")
	}

	if !statefulSetEqual(sfs, &expSS) {
		state = statusUpdated
		expSS.SetResourceVersion(sfs.GetResourceVersion())
		_, err = sfsClient.Update(&expSS)
		return err
	}

	if !reflect.DeepEqual(sfs.Spec, expSS.Spec) {
		state = statusSkip
		glog.V(2).Infof("statefulSet has changes that can't be applied")
	}

	return nil
}

func (f *cFactory) syncUtilitySecret() error {
	expS := f.createUtilitySecret()
	state := statusOk
	defer func() {
		glog.V(2).Infof("utility secret...(%s)", state)
	}()

	if _, err := kube.EnsureSecretKeys(f.client, expS, true); err != nil {
		state = statusFaild
		return fmt.Errorf("fail to ensure secret: %s", err)
	}
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
