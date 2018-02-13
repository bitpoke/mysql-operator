package mysqlcluster

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"k8s.io/api/apps/v1"
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

type component struct {
	name   string
	syncFn func() (string, error)
	//event reason when sync faild
	erFaild string
	// event reason when boject is modified
	erUpdated string
}

func (f *cFactory) getComponents() []component {
	return []component{
		component{
			name:      fmt.Sprintf("db-credentials(%s)", f.cl.Spec.SecretName),
			syncFn:    f.syncDbCredentialsSecret,
			erFaild:   api.EventReasonDbSecretFaild,
			erUpdated: api.EventReasonDbSecretUpdated,
		},
		component{
			name:      f.getNameForResource(UtilitySecret),
			syncFn:    f.syncUtilitySecret,
			erFaild:   api.EventReasonUtilitySecretFaild,
			erUpdated: api.EventReasonUtilitySecretUpdated,
		},
		component{
			name:      f.getNameForResource(EnvSecret),
			syncFn:    f.syncConfigEnvSecret,
			erFaild:   api.EventReasonEnvSecretFaild,
			erUpdated: api.EventReasonEnvSecretUpdated,
		},
		component{
			name:      f.getNameForResource(ConfigMap),
			syncFn:    f.syncConfigMapFiles,
			erFaild:   api.EventReasonConfigMapFaild,
			erUpdated: api.EventReasonConfigMapUpdated,
		},
		component{
			name:      f.getNameForResource(HeadlessSVC),
			syncFn:    f.syncHeadlessService,
			erFaild:   api.EventReasonServiceFaild,
			erUpdated: api.EventReasonServiceUpdated,
		},
		component{
			name:      f.getNameForResource(StatefulSet),
			syncFn:    f.syncStatefulSet,
			erFaild:   api.EventReasonSFSFaild,
			erUpdated: api.EventReasonSFSUpdated,
		},
	}
}

func (f *cFactory) Sync(ctx context.Context) error {
	if len(f.cl.Spec.SecretName) == 0 {
		err := fmt.Errorf("the Spec.SecretName is empty")
		f.rec.Eventf(f.cl, api.EventWarning, api.EventReasonDbSecretFaild,
			"faild to sync db-credentials secret: %s", err)
		return err
	}

	for _, comp := range f.getComponents() {
		state, err := comp.syncFn()
		if err != nil {
			glog.V(2).Infof("%s ... (%s)", comp.name, state)
			err = fmt.Errorf("%s faild to sync with err: %s", comp.name, err)
			f.rec.Event(f.cl, api.EventWarning, comp.erFaild, err.Error())
			return err
		}
		glog.V(2).Infof("%s ... (%s)", comp.name, state)
		switch state {
		case statusCreated, statusUpdated:
			f.rec.Event(f.cl, api.EventNormal, comp.erUpdated, "")
		}
	}
	return nil
}

func (f *cFactory) syncHeadlessService() (state string, err error) {
	expHL := f.createHeadlessService()
	state = statusUpToDate

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
		err = fmt.Errorf("the selected resource not controlled by me")
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

	scrtClient := f.client.CoreV1().Secrets(f.namespace)
	cfgSct, err := scrtClient.Get(f.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = scrtClient.Create(&expCS)
		return
	} else if err != nil {
		state = statusFaild
		return
	}
	if !metav1.IsControlledBy(cfgSct, f.cl) {
		// log a warning to evant recorder...
		state = statusFaild
		err = fmt.Errorf("the selected resource not controlled by me")
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

	cmClient := f.client.CoreV1().ConfigMaps(f.namespace)
	cfgMap, err := cmClient.Get(f.getNameForResource(ConfigMap), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		state = statusCreated
		_, err = cmClient.Create(&expCM)
		return
	} else if err != nil {
		state = statusFaild
		return
	}

	if !metav1.IsControlledBy(cfgMap, f.cl) {
		state = statusSkip
		err = fmt.Errorf("the selected resource not controlled by me")
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

	sfsClient := f.client.AppsV1().StatefulSets(f.namespace)
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
		err = fmt.Errorf("the selected resource not controlled by me")
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

	sfs, err = sfsClient.Get(f.getNameForResource(StatefulSet), metav1.GetOptions{})
	if err != nil {
		state = statusFaild
		err = fmt.Errorf("error getting second time statefulset: %s", err)
		return
	}
	if sfs.Status.ReadyReplicas == sfs.Status.Replicas {
		f.cl.UpdateStatusCondition(api.ClusterConditionReady,
			apiv1.ConditionTrue, "statefulset ready", "Cluster is ready.")
	} else {
		f.cl.UpdateStatusCondition(api.ClusterConditionReady,
			apiv1.ConditionFalse, "statefulset not ready", "Cluster is not ready.")
	}
	return
}

func (f *cFactory) syncUtilitySecret() (state string, err error) {
	expS := f.createUtilitySecret()
	state = statusOk

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

func statefulSetEqual(a, b *v1.StatefulSet) bool {
	if *a.Spec.Replicas != *b.Spec.Replicas {
		return false
	}
	return true
}
