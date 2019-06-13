package utils

import (
	"context"

	"github.com/pivotal-cf/brokerapi/domain"
)

type contextKey string

const (
	contextKeyService contextKey = "brokerapi_service"
	contextKeyPlan    contextKey = "brokerapi_plan"
)

func AddServiceToContext(ctx context.Context, service *domain.Service) context.Context {
	if service != nil {
		return context.WithValue(ctx, contextKeyService, service)
	}
	return ctx
}

func RetrieveServiceFromContext(ctx context.Context) *domain.Service {
	if value := ctx.Value(contextKeyService); value != nil {
		return value.(*domain.Service)
	}
	return nil
}

func AddServicePlanToContext(ctx context.Context, plan *domain.ServicePlan) context.Context {
	if plan != nil {
		return context.WithValue(ctx, contextKeyPlan, plan)
	}
	return ctx
}

func RetrieveServicePlanFromContext(ctx context.Context) *domain.ServicePlan {
	if value := ctx.Value(contextKeyPlan); value != nil {
		return value.(*domain.ServicePlan)
	}
	return nil
}
