package mysqlcluster

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
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
			name:      f.getNameForResource(EnvSecret),
			syncFn:    f.syncEnvSecret,
			erFaild:   api.EventReasonEnvSecretFaild,
			erUpdated: api.EventReasonEnvSecretUpdated,
		},
		component{
			name:      f.getNameForResource(ConfigMap),
			syncFn:    f.syncConfigMysqlMap,
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
