package fakes

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi"
)

type FakeServiceBroker struct {
	ProvisionDetails   brokerapi.ProvisionDetails
	UpdateDetails      brokerapi.UpdateDetails
	DeprovisionDetails brokerapi.DeprovisionDetails

	ProvisionedInstanceIDs   []string
	DeprovisionedInstanceIDs []string
	UpdatedInstanceIDs       []string
	GetInstanceIDs           []string

	BoundInstanceIDs    []string
	BoundBindingIDs     []string
	BoundBindingDetails brokerapi.BindDetails
	SyslogDrainURL      string
	RouteServiceURL     string
	VolumeMounts        []brokerapi.VolumeMount

	UnbindingDetails brokerapi.UnbindDetails

	InstanceLimit int

	ProvisionError     error
	BindError          error
	UnbindError        error
	DeprovisionError   error
	LastOperationError error
	UpdateError        error
	GetInstanceError   error
	GetBindingError    error

	BrokerCalled             bool
	LastOperationState       brokerapi.LastOperationState
	LastOperationDescription string

	AsyncAllowed bool

	ShouldReturnAsync     bool
	DashboardURL          string
	OperationDataToReturn string

	LastOperationInstanceID string
	LastOperationData       string

	ReceivedContext bool

	ServiceID string
	PlanID    string
}

type FakeAsyncServiceBroker struct {
	FakeServiceBroker
	ShouldProvisionAsync bool
}

type FakeAsyncOnlyServiceBroker struct {
	FakeServiceBroker
}

func (fakeBroker *FakeServiceBroker) Services(ctx context.Context) ([]brokerapi.Service, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := ctx.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if val, ok := ctx.Value("fails").(bool); ok && val {
		return []brokerapi.Service{}, errors.New("something went wrong!")
	}

	return []brokerapi.Service{
		{
			ID:            fakeBroker.ServiceID,
			Name:          "p-cassandra",
			Description:   "Cassandra service for application development and testing",
			Bindable:      true,
			PlanUpdatable: true,
			Plans: []brokerapi.ServicePlan{
				{
					ID:          fakeBroker.PlanID,
					Name:        "default",
					Description: "The default Cassandra plan",
					Metadata: &brokerapi.ServicePlanMetadata{
						Bullets:     []string{},
						DisplayName: "Cassandra",
					},
					MaintenanceInfo: &brokerapi.MaintenanceInfo{
						Public: map[string]string{
							"name": "foo",
						},
					},
					Schemas: &brokerapi.ServiceSchemas{
						Instance: brokerapi.ServiceInstanceSchema{
							Create: brokerapi.Schema{
								Parameters: map[string]interface{}{
									"$schema": "http://json-schema.org/draft-04/schema#",
									"type":    "object",
									"properties": map[string]interface{}{
										"billing-account": map[string]interface{}{
											"description": "Billing account number used to charge use of shared fake server.",
											"type":        "string",
										},
									},
								},
							},
							Update: brokerapi.Schema{
								Parameters: map[string]interface{}{
									"$schema": "http://json-schema.org/draft-04/schema#",
									"type":    "object",
									"properties": map[string]interface{}{
										"billing-account": map[string]interface{}{
											"description": "Billing account number used to charge use of shared fake server.",
											"type":        "string",
										},
									},
								},
							},
						},
						Binding: brokerapi.ServiceBindingSchema{
							Create: brokerapi.Schema{
								Parameters: map[string]interface{}{
									"$schema": "http://json-schema.org/draft-04/schema#",
									"type":    "object",
									"properties": map[string]interface{}{
										"billing-account": map[string]interface{}{
											"description": "Billing account number used to charge use of shared fake server.",
											"type":        "string",
										},
									},
								},
							},
						},
					},
				},
			},
			Metadata: &brokerapi.ServiceMetadata{
				DisplayName:      "Cassandra",
				LongDescription:  "Long description",
				DocumentationUrl: "http://thedocs.com",
				SupportUrl:       "http://helpme.no",
			},
			Tags: []string{
				"pivotal",
				"cassandra",
			},
		},
	}, nil
}

func (fakeBroker *FakeServiceBroker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.ProvisionError != nil {
		return brokerapi.ProvisionedServiceSpec{}, fakeBroker.ProvisionError
	}

	if len(fakeBroker.ProvisionedInstanceIDs) >= fakeBroker.InstanceLimit {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceLimitMet
	}

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceAlreadyExists
	}

	fakeBroker.ProvisionDetails = details
	fakeBroker.ProvisionedInstanceIDs = append(fakeBroker.ProvisionedInstanceIDs, instanceID)
	return brokerapi.ProvisionedServiceSpec{DashboardURL: fakeBroker.DashboardURL}, nil
}

func (fakeBroker *FakeAsyncServiceBroker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if fakeBroker.ProvisionError != nil {
		return brokerapi.ProvisionedServiceSpec{}, fakeBroker.ProvisionError
	}

	if len(fakeBroker.ProvisionedInstanceIDs) >= fakeBroker.InstanceLimit {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceLimitMet
	}

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceAlreadyExists
	}

	fakeBroker.ProvisionDetails = details
	fakeBroker.ProvisionedInstanceIDs = append(fakeBroker.ProvisionedInstanceIDs, instanceID)
	return brokerapi.ProvisionedServiceSpec{IsAsync: fakeBroker.ShouldProvisionAsync, DashboardURL: fakeBroker.DashboardURL, OperationData: fakeBroker.OperationDataToReturn}, nil
}

func (fakeBroker *FakeAsyncOnlyServiceBroker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if fakeBroker.ProvisionError != nil {
		return brokerapi.ProvisionedServiceSpec{}, fakeBroker.ProvisionError
	}

	if len(fakeBroker.ProvisionedInstanceIDs) >= fakeBroker.InstanceLimit {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceLimitMet
	}

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceAlreadyExists
	}

	if !asyncAllowed {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrAsyncRequired
	}

	fakeBroker.ProvisionDetails = details
	fakeBroker.ProvisionedInstanceIDs = append(fakeBroker.ProvisionedInstanceIDs, instanceID)
	return brokerapi.ProvisionedServiceSpec{IsAsync: true, DashboardURL: fakeBroker.DashboardURL}, nil
}

func (fakeBroker *FakeServiceBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.UpdateError != nil {
		return brokerapi.UpdateServiceSpec{}, fakeBroker.UpdateError
	}

	fakeBroker.UpdateDetails = details
	fakeBroker.UpdatedInstanceIDs = append(fakeBroker.UpdatedInstanceIDs, instanceID)
	fakeBroker.AsyncAllowed = asyncAllowed
	return brokerapi.UpdateServiceSpec{IsAsync: fakeBroker.ShouldReturnAsync, OperationData: fakeBroker.OperationDataToReturn, DashboardURL: fakeBroker.DashboardURL}, nil
}

func (fakeBroker *FakeServiceBroker) GetInstance(context context.Context, instanceID string) (brokerapi.GetInstanceDetailsSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	fakeBroker.GetInstanceIDs = append(fakeBroker.GetInstanceIDs, instanceID)
	return brokerapi.GetInstanceDetailsSpec{
		ServiceID:    fakeBroker.ServiceID,
		PlanID:       fakeBroker.PlanID,
		DashboardURL: fakeBroker.DashboardURL,
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
	}, fakeBroker.GetInstanceError
}

func (fakeBroker *FakeServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.DeprovisionError != nil {
		return brokerapi.DeprovisionServiceSpec{}, fakeBroker.DeprovisionError
	}

	fakeBroker.DeprovisionDetails = details
	fakeBroker.DeprovisionedInstanceIDs = append(fakeBroker.DeprovisionedInstanceIDs, instanceID)

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.DeprovisionServiceSpec{}, nil
	}
	return brokerapi.DeprovisionServiceSpec{IsAsync: false}, brokerapi.ErrInstanceDoesNotExist
}

func (fakeBroker *FakeAsyncOnlyServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if fakeBroker.DeprovisionError != nil {
		return brokerapi.DeprovisionServiceSpec{IsAsync: true}, fakeBroker.DeprovisionError
	}

	if !asyncAllowed {
		return brokerapi.DeprovisionServiceSpec{IsAsync: true}, brokerapi.ErrAsyncRequired
	}

	fakeBroker.DeprovisionedInstanceIDs = append(fakeBroker.DeprovisionedInstanceIDs, instanceID)
	fakeBroker.DeprovisionDetails = details

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.DeprovisionServiceSpec{IsAsync: true, OperationData: fakeBroker.OperationDataToReturn}, nil
	}

	return brokerapi.DeprovisionServiceSpec{IsAsync: true, OperationData: fakeBroker.OperationDataToReturn}, brokerapi.ErrInstanceDoesNotExist
}

func (fakeBroker *FakeAsyncServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	fakeBroker.BrokerCalled = true

	if fakeBroker.DeprovisionError != nil {
		return brokerapi.DeprovisionServiceSpec{IsAsync: asyncAllowed}, fakeBroker.DeprovisionError
	}

	fakeBroker.DeprovisionedInstanceIDs = append(fakeBroker.DeprovisionedInstanceIDs, instanceID)
	fakeBroker.DeprovisionDetails = details

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		return brokerapi.DeprovisionServiceSpec{IsAsync: asyncAllowed, OperationData: fakeBroker.OperationDataToReturn}, nil
	}

	return brokerapi.DeprovisionServiceSpec{OperationData: fakeBroker.OperationDataToReturn, IsAsync: asyncAllowed}, brokerapi.ErrInstanceDoesNotExist
}

func (fakeBroker *FakeServiceBroker) GetBinding(context context.Context, instanceID, bindingID string) (brokerapi.GetBindingSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	return brokerapi.GetBindingSpec{
		Credentials: FakeCredentials{
			Host:     "127.0.0.1",
			Port:     3000,
			Username: "batman",
			Password: "robin",
		},
		SyslogDrainURL:  fakeBroker.SyslogDrainURL,
		RouteServiceURL: fakeBroker.RouteServiceURL,
		VolumeMounts:    fakeBroker.VolumeMounts,
	}, fakeBroker.GetBindingError
}

func (fakeBroker *FakeAsyncServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	fakeBroker.BrokerCalled = true

	fakeBroker.BoundBindingDetails = details

	fakeBroker.BoundInstanceIDs = append(fakeBroker.BoundInstanceIDs, instanceID)
	fakeBroker.BoundBindingIDs = append(fakeBroker.BoundBindingIDs, bindingID)

	if asyncAllowed {
		return brokerapi.Binding{
			IsAsync:       true,
			OperationData: "0xDEADBEEF",
		}, nil
	} else {
		return fakeBroker.FakeServiceBroker.Bind(context, instanceID, bindingID, details, false)
	}
}

func (fakeBroker *FakeServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.BindError != nil {
		return brokerapi.Binding{}, fakeBroker.BindError
	}

	fakeBroker.BoundBindingDetails = details

	fakeBroker.BoundInstanceIDs = append(fakeBroker.BoundInstanceIDs, instanceID)
	fakeBroker.BoundBindingIDs = append(fakeBroker.BoundBindingIDs, bindingID)

	return brokerapi.Binding{
		Credentials: FakeCredentials{
			Host:     "127.0.0.1",
			Port:     3000,
			Username: "batman",
			Password: "robin",
		},
		SyslogDrainURL:  fakeBroker.SyslogDrainURL,
		RouteServiceURL: fakeBroker.RouteServiceURL,
		VolumeMounts:    fakeBroker.VolumeMounts,
	}, nil
}

func (fakeBroker *FakeServiceBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	fakeBroker.BrokerCalled = true

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.UnbindError != nil {
		return brokerapi.UnbindSpec{}, fakeBroker.UnbindError
	}

	fakeBroker.UnbindingDetails = details

	if sliceContains(instanceID, fakeBroker.ProvisionedInstanceIDs) {
		if sliceContains(bindingID, fakeBroker.BoundBindingIDs) {
			return brokerapi.UnbindSpec{}, nil
		}
		return brokerapi.UnbindSpec{}, brokerapi.ErrBindingDoesNotExist
	}

	return brokerapi.UnbindSpec{}, brokerapi.ErrInstanceDoesNotExist
}

func (fakeBroker *FakeServiceBroker) LastBindingOperation(context context.Context, instanceID, bindingID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.LastOperationError != nil {
		return brokerapi.LastOperation{}, fakeBroker.LastOperationError
	}

	return brokerapi.LastOperation{State: fakeBroker.LastOperationState, Description: fakeBroker.LastOperationDescription}, nil
}

func (fakeBroker *FakeServiceBroker) LastOperation(context context.Context, instanceID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	fakeBroker.LastOperationInstanceID = instanceID
	fakeBroker.LastOperationData = details.OperationData

	if val, ok := context.Value("test_context").(bool); ok {
		fakeBroker.ReceivedContext = val
	}

	if fakeBroker.LastOperationError != nil {
		return brokerapi.LastOperation{}, fakeBroker.LastOperationError
	}

	return brokerapi.LastOperation{State: fakeBroker.LastOperationState, Description: fakeBroker.LastOperationDescription}, nil
}

type FakeCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func sliceContains(needle string, haystack []string) bool {
	for _, element := range haystack {
		if element == needle {
			return true
		}
	}
	return false
}
