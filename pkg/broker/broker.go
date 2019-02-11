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
	"fmt"
	"net/http"

	"github.com/pivotal-cf/brokerapi"
	"go.uber.org/zap"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	logf "github.com/presslabs/controller-util/log"

	"github.com/presslabs/controller-util/log/adapters/lager"
)

var log = logf.Log.WithName("service-broker")

type BrokerServer struct { //nolint: golint
	HTTPServer *http.Server
}

type serviceBroker struct {
	client.Client
}

func (sb *serviceBroker) Services(ctx context.Context) (services []brokerapi.Service, err error) {
	services = []brokerapi.Service{
		{
			Name: "mysql",
			ID:   "79f8df87-658d-4056-803b-d66c17b6e437",
		},
	}
	return services, nil
}

func (sb *serviceBroker) Provision(
	ctx context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (spec brokerapi.ProvisionedServiceSpec, err error) {
	return spec, err
}

func (sb *serviceBroker) Deprovision(
	ctx context.Context,
	instanceID string,
	details brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (spec brokerapi.DeprovisionServiceSpec, err error) {
	return spec, err
}

func (sb *serviceBroker) GetInstance(
	ctx context.Context,
	instanceID string,
) (spec brokerapi.GetInstanceDetailsSpec, err error) {
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

func (sb *serviceBroker) Bind(
	ctx context.Context,
	instanceID, bindingID string,
	details brokerapi.BindDetails,
	asyncAllowed bool,
) (binding brokerapi.Binding, err error) {
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

func NewBrokerServer(addr string, mgr manager.Manager) *BrokerServer { // nolint: golint
	h := brokerapi.New(
		&serviceBroker{Client: mgr.GetClient()},
		lager.NewZapAdapter("broker", zap.L()),
		brokerapi.BrokerCredentials{
			Username: "calin",
			Password: "lemon",
		},
	)

	broker := &BrokerServer{}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: broker.Log(h.ServeHTTP),
	}

	return &BrokerServer{
		HTTPServer: httpServer,
	}
}

func (s *BrokerServer) Log(h http.HandlerFunc) http.HandlerFunc { // nolint: golint
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf(">>>>>>>>>>>>>>>>>>> %#v\n", r)
		h(w, r)
	}
}

func (s *BrokerServer) Start(stop <-chan struct{}) error { // nolint: golint
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

func AddToManager(mgr manager.Manager) error { // nolint: golint
	srv := NewBrokerServer(":8080", mgr)
	return mgr.Add(srv)
}
