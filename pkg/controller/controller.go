package controller

import (
	"time"

	"github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	"github.com/presslabs/titanium/pkg/util/k8sutil"
)

var initRetryWaitTime = 30 * time.Second

type Event struct {
	Type kwatch.EventType
	//Object *api.EtcdCluster
}

type Controller struct {
	logger *logrus.Entry
	Config

	//clusters map[string]*cluster.Cluster
}

type Config struct {
	Namespace      string
	ServiceAccount string
	KubeCli        kubernetes.Interface
	KubeExtCli     apiextensionsclient.Interface
	CreateCRD      bool
}

func New(cfg Config) *Controller {
	return &Controller{
		logger: logrus.WithField("pkg", "controller"),

		Config: cfg,
		//clusters: make(map[string]*cluster.Cluster),
	}
}

func (c *Controller) Start() error {
	c.logger.Info("Starting controller ...")

	if !c.Config.CreateCRD {
		err := c.createCRDIfNotExists()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) createCRDIfNotExists() error {
	c.logger.Info("Creating CRD...")

	err := k8sutil.CreateCRD(
		c.KubeExtCli,
		api.MysqlClusterCRDName,
		api.MysqlClusterCRDKind,
		api.MysqlClusterCRDPlural,
		"mysql",
	)
	if err != nil {
		c.logger.Error("Faild to create CRD: %v", err)
		return err
	}
	return k8sutil.WaitCRDReady(c.KubeExtCli, api.MysqlClusterCRDName)
}
