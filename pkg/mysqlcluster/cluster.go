package mysqlcluster

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
)

type Interface interface {
	Sync(ctx context.Context) error

	//	Create(ctx context.Context) error
	//	Update(ctx context.Context) error
	//	Delete(ctx context.Context) error
}

type cluster struct {
	cl *api.MysqlCluster

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
	return &cluster{
		cl:        cl,
		client:    klient,
		cmclient:  cmclient,
		namespace: ns,
	}
}

func (c *cluster) Sync(ctx context.Context) error {
	if err := c.syncHeadlessService(); err != nil {
		return err
	}
	if err := c.syncConfigEnvSecret(); err != nil {
		return err
	}
	if err := c.syncConfigMapFiles(); err != nil {
		return err
	}
	if err := c.syncStatefulSet(); err != nil {
		return err
	}
	return nil
}

func (c *cluster) syncHeadlessService() error {
	expHL := c.createHeadlessService()
	fmt.Printf("Synced Headless Service ... ")
	msg := "up-to-date"

	servicesClient := c.client.CoreV1().Services(c.namespace)
	hlSVC, err := servicesClient.Get(c.getNameForResource(HeadlessSVC), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		msg = "created"
		if _, err = servicesClient.Create(&expHL); err != nil {
			return err
		}
	}
	if !metav1.IsControlledBy(hlSVC, c.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The service is not controlled by this resource!")
	}

	// TODO: find a better condition
	if !reflect.DeepEqual(hlSVC.Spec.Ports, expHL.Spec.Ports) {
		msg = "updated"
		expHL.SetResourceVersion(hlSVC.GetResourceVersion())
		if _, err := servicesClient.Update(&expHL); err != nil {
			return err
		}
	}

	fmt.Printf("%s\n", msg)
	return nil
}

func (c *cluster) syncConfigEnvSecret() error {
	expCS := c.createEnvConfigSecret()
	fmt.Printf("Syncing Env Secret ... ")
	msg := "up-to-date"

	scrtClient := c.client.CoreV1().Secrets(c.namespace)
	cfgSct, err := scrtClient.Get(c.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		msg = "created"
		if _, err = scrtClient.Create(&expCS); err != nil {
			return err
		}
	}
	if !metav1.IsControlledBy(cfgSct, c.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config env map is not controlled by this resource!")
	}

	if !reflect.DeepEqual(cfgSct.Data, expCS.Data) {
		msg = "updated"
		expCS.SetResourceVersion(cfgSct.GetResourceVersion())
		if _, err := scrtClient.Update(&expCS); err != nil {
			return nil
		}
	}

	fmt.Printf("%s\n", msg)
	return nil
}

func (c *cluster) syncConfigMapFiles() error {
	expCM := c.createConfigMapFiles()
	fmt.Printf("Syncing ConfigMap ... ")
	msg := "up-to-date"

	cmClient := c.client.CoreV1().ConfigMaps(c.namespace)
	cfgMap, err := cmClient.Get(c.getNameForResource(ConfigMap), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if _, err = cmClient.Create(&expCM); err != nil {
			return nil
		}
	}
	if !metav1.IsControlledBy(cfgMap, c.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config map files is not controlled by this resource!")
	}

	// TODO: fix: Data is the same but not in the same order.
	if !reflect.DeepEqual(cfgMap.Data, expCM.Data) {
		msg = "updated"
		expCM.SetResourceVersion(cfgMap.GetResourceVersion())
		if _, err := cmClient.Update(&expCM); err != nil {
			return nil
		}
	}

	fmt.Printf("%s\n", msg)
	return nil
}

func (c *cluster) syncStatefulSet() error {
	expSS := c.createStatefulSet()
	fmt.Printf("Syncing StatefullSet ... ")
	msg := "up-to-date"

	sfsClient := c.client.AppsV1beta2().StatefulSets(c.namespace)
	sfs, err := sfsClient.Get(c.getNameForResource(StatefulSet), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if _, err = sfsClient.Create(&expSS); err != nil {
			return err
		}
	}
	if !metav1.IsControlledBy(sfs, c.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The config map files is not controlled by this resource!")
	}

	// TODO: add update condition here
	msg = "updated"
	expSS.SetResourceVersion(sfs.GetResourceVersion())
	if _, err = sfsClient.Update(&expSS); err != nil {
		return err
	}

	fmt.Printf("%s\n", msg)
	return err
}

func (c *cluster) getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		c.cl.AsOwnerReference(),
	}
}
