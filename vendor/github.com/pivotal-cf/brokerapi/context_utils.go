package brokerapi

import (
	"context"

	"github.com/pivotal-cf/brokerapi/utils"
)

func AddServiceToContext(ctx context.Context, service *Service) context.Context {
	return utils.AddServiceToContext(ctx, service)
}

func RetrieveServiceFromContext(ctx context.Context) *Service {
	return utils.RetrieveServiceFromContext(ctx)
}

func AddServicePlanToContext(ctx context.Context, plan *ServicePlan) context.Context {
	return utils.AddServicePlanToContext(ctx, plan)
}

func RetrieveServicePlanFromContext(ctx context.Context) *ServicePlan {
	return utils.RetrieveServicePlanFromContext(ctx)
}
