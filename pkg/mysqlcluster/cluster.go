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

	servicesClient := c.client.CoreV1().Services(c.namespace)
	hlSVC, err := servicesClient.Get(c.getNameForResource(HeadlessSVC), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Println("HeadlessService ... created")
		_, err = servicesClient.Create(&expHL)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(hlSVC, c.cl) {
		// log a warning to evant recorder...
		return fmt.Errorf("The service is not controlled by this resource!")
	}

	// TODO: find a better condition
	if !reflect.DeepEqual(hlSVC.Spec.Ports, expHL.Spec.Ports) {
		fmt.Println("HeadlessService ... updated")
		expHL.SetResourceVersion(hlSVC.GetResourceVersion())
		_, err := servicesClient.Update(&expHL)
		return err
	}

	fmt.Println("HeadlessService ... up-to-data")
	return nil
}

func (c *cluster) syncConfigEnvSecret() error {
	expCS := c.createEnvConfigSecret()

	scrtClient := c.client.CoreV1().Secrets(c.namespace)
	cfgSct, err := scrtClient.Get(c.getNameForResource(EnvSecret), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Println("ConfSecret ... created")
		_, err = scrtClient.Create(&expCS)
		return err
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(cfgSct, c.cl) {
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

func (c *cluster) syncConfigMapFiles() error {
	expCM := c.createConfigMapFiles()
	cmClient := c.client.CoreV1().ConfigMaps(c.namespace)
	cfgMap, err := cmClient.Get(c.getNameForResource(ConfigMap), metav1.GetOptions{})

	if errors.IsNotFound(err) {
		fmt.Println("ConfigMap ... created")
		_, err = cmClient.Create(&expCM)
		return err
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(cfgMap, c.cl) {
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

func (c *cluster) syncStatefulSet() error {
	expSS := c.createStatefulSet()

	sfsClient := c.client.AppsV1beta2().StatefulSets(c.namespace)
	sfs, err := sfsClient.Get(c.getNameForResource(StatefulSet), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Println("StatefulSet ... created")
		_, err = sfsClient.Create(&expSS)
		return err
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(sfs, c.cl) {
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

func (c *cluster) getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		c.cl.AsOwnerReference(),
	}
}

// This method checks if given secret exists in cluster namespace.
func (c *cluster) existsSecret(name string) bool {
	if len(name) == 0 {
		return false
	}

	client := c.client.CoreV1().Secrets(c.namespace)
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
