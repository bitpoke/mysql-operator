/*
Copyright 2019 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqldsl"
	"net/http"

	broker "github.com/pivotal-cf/brokerapi"
	brokerapi "github.com/pivotal-cf/brokerapi/domain"

	"github.com/gorilla/mux"
	logf "github.com/presslabs/controller-util/log"
	"github.com/presslabs/controller-util/log/adapters/lager"
	"github.com/presslabs/controller-util/rand"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var log = logf.Log.WithName("service-broker")

const instanceIDLabel = "openservicebroker.presslabs.org/instanceID"

// Server for service broker
type Server struct {
	HTTPServer *http.Server
}

type serviceBroker struct {
	client.Client
}

func (sb *serviceBroker) Services(ctx context.Context) (services []brokerapi.Service, err error) {
	services = []brokerapi.Service{
		{
			Name:        MysqlServiceName,
			ID:          MysqlServiceID,
			Description: MysqlServiceDescription,
			Plans:       sb.plans(MysqlServiceID),
		},
	}
	return services, nil
}

// MySQLProvisionParameters represents the parameters that can be tuned on a cluster
type MySQLProvisionParameters struct {
	// Name of the MySQL cluster resource
	Name string

	// Replicas represents the number of nodes for MySQL cluster
	Replicas int32

	//MySQLVersion is the MySQL server version
	MySQLVersion string

	// Size represents the size. Example: 1Gi
	Size string
}

func (sb *serviceBroker) Provision(
	ctx context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (spec brokerapi.ProvisionedServiceSpec, err error) {
	// this action is async
	spec.IsAsync = true

	// Get the provision parameters
	namespace := getNamespaceFromContext(details.RawContext)
	params := MySQLProvisionParameters{}
	err = json.Unmarshal(details.GetRawParameters(), &params)
	if err != nil {
		return spec, err
	}
	random, err := rand.AlphaNumericString(16)
	if err != nil {
		return spec, err
	}

	// create the secret for MySQL cluster with the root password random generated
	secretName := fmt.Sprintf("%s-credentials", params.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"ROOT_PASSWORD": []byte(random),
		},
	}

	err = sb.Client.Create(ctx, secret)
	if err != nil {
		return spec, err
	}

	size, err := resource.ParseQuantity(params.Size)
	if err != nil {
		return spec, err
	}

	clSpec := api.MysqlClusterSpec{}
	clSpec.SecretName = secretName
	clSpec.Replicas = &params.Replicas
	clSpec.MysqlVersion = params.MySQLVersion
	clSpec.VolumeSpec = api.VolumeSpec{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}

	// create the cluster
	cluster := &api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: namespace,
			Labels: map[string]string{
				instanceIDLabel: instanceID,
			},
		},
		Spec: clSpec,
	}

	err = sb.Client.Create(ctx, cluster)
	return spec, err
}

func (sb *serviceBroker) Deprovision(
	ctx context.Context,
	instanceID string,
	details brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (spec brokerapi.DeprovisionServiceSpec, err error) {
	// this action is async
	spec.IsAsync = true

	// Get the provision parameters
	cluster, err := sb.getClusterForID(ctx, instanceID)
	if err != nil {
		return spec, err
	}

	// cluster is deleting
	if cluster.DeletionTimestamp != nil {
		return spec, nil
	}

	// delete the cluster
	err = sb.Client.Delete(ctx, cluster)

	return spec, err
}

func (sb *serviceBroker) GetInstance(
	ctx context.Context,
	instanceID string,
) (spec brokerapi.GetInstanceDetailsSpec, err error) {

	// get the cluster and set the cluster spec to the parameters list
	cluster, err := sb.getClusterForID(ctx, instanceID)
	spec.Parameters = cluster.Spec
	return spec, err
}

func (sb *serviceBroker) Update(
	ctx context.Context,
	instanceID string,
	details brokerapi.UpdateDetails,
	asyncAllowed bool,
) (spec brokerapi.UpdateServiceSpec, err error) {
	return spec, err
}

func (sb *serviceBroker) LastOperation(
	ctx context.Context,
	instanceID string,
	details brokerapi.PollDetails,
) (op brokerapi.LastOperation, err error) {
	return op, err
}

// MySQLBindParameters represents the parameters that are passed when binding a new instance
type MySQLBindParameters struct {
	// Database: the database name
	Database string

	// User: the username
	User string
}

func (sb *serviceBroker) Bind(
	ctx context.Context,
	instanceID, bindingID string,
	details brokerapi.BindDetails,
	asyncAllowed bool,
) (binding brokerapi.Binding, err error) {

	// get the MySQL cluster
	cluster, err := sb.getClusterForID(ctx, instanceID)
	if err != nil {
		return binding, err
	}

	// establish a mysql connection
	user, pass, err := sb.getUtilityUserPassword(ctx, cluster)
	conn := newConnection(cluster, user, pass)
	err = conn.Open()
	if err != nil {
		return binding, err
	}
	defer func() {
		if cErr := conn.Close(); cErr != nil {
			log.Error(cErr, "mysql can't close connection")
		}
	}()

	// get bind details, parameters
	params := MySQLBindParameters{}
	err = json.Unmarshal(details.GetRawParameters(), &params)
	if err != nil {
		return binding, err
	}

	// generate a new password for new user
	uPass, err := rand.AlphaNumericString(15)
	if err != nil {
		return binding, err
	}

	// create database
	err = conn.RunQueries(ctx, []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", params.Database),
	})
	if err != nil {
		return binding, err
	}

	// create user
	err = conn.RunQueries(ctx,
		mysqldsl.CreateUserQuery(params.Database, uPass, "%",
			[]string{"ALL"}, fmt.Sprintf("%s.*", params.Database)),
	)
	if err != nil {
		return binding, err
	}

	return binding, err
}

func (sb *serviceBroker) Unbind(
	ctx context.Context,
	instanceID, bindingID string,
	details brokerapi.UnbindDetails,
	asyncAllowed bool,
) (spec brokerapi.UnbindSpec, err error) {
	return spec, err
}

func (sb *serviceBroker) GetBinding(
	ctx context.Context,
	instanceID, bindingID string,
) (spec brokerapi.GetBindingSpec, err error) {
	return spec, err
}

func (sb *serviceBroker) LastBindingOperation(
	ctx context.Context,
	instanceID, bindingID string,
	details brokerapi.PollDetails,
) (op brokerapi.LastOperation, err error) {
	return op, err
}

var _ brokerapi.ServiceBroker = &serviceBroker{}

// NewBrokerServer returns a HTTP server with service broker API implemented
func NewBrokerServer(addr string, mgr manager.Manager) *Server {
	// h := brokerapi.New(
	// 	&serviceBroker{Client: mgr.GetClient()},
	// 	lager.NewZapAdapter("broker", zap.L()),
	// 	brokerapi.BrokerCredentials{
	// 		Username: "",
	// 		Password: "",
	// 	},
	// )

	router := mux.NewRouter()
	broker.AttachRoutes(router, &serviceBroker{Client: mgr.GetClient()}, lager.NewZapAdapter("broker", zap.L()))

	broker := &Server{}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: broker.log(router.ServeHTTP),
	}

	return &Server{
		HTTPServer: httpServer,
	}
}

func (s *Server) log(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf(">>>>>>>>>>>>>>>>>>> %#v\n", r)
		h(w, r)
	}
}

// Start starts the broker http server
func (s *Server) Start(stop <-chan struct{}) error {
	errChan := make(chan error, 1)
	go func() {
		log.Info("Web Server listening", "address", s.HTTPServer.Addr)
		if err := s.HTTPServer.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-stop:
		if err := s.HTTPServer.Shutdown(context.TODO()); err != nil {
			log.Error(err, "unable to shutdown HTTP server properly")
		}
	case err := <-errChan:
		return err
	}
	return nil
}

// AddToManager registers the new mysql broker to manager
func AddToManager(mgr manager.Manager) error {
	srv := NewBrokerServer(":8080", mgr)
	return mgr.Add(srv)
}
