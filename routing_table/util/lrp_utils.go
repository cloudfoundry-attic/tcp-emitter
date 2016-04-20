package util

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	"github.com/pivotal-golang/lager"
)

func DesiredLRPData(lrp *models.DesiredLRP) lager.Data {
	logRoutes := make(models.Routes)
	logRoutes[cfroutes.CF_ROUTER] = (*lrp.Routes)[cfroutes.CF_ROUTER]
	logRoutes[tcp_routes.TCP_ROUTER] = (*lrp.Routes)[tcp_routes.TCP_ROUTER]

	return lager.Data{
		"process-guid": lrp.ProcessGuid,
		"routes":       logRoutes,
		"ports":        lrp.Ports,
	}
}

func ActualLRPData(lrp *models.ActualLRP, evacuating bool) lager.Data {
	return lager.Data{
		"process-guid":  lrp.ProcessGuid,
		"index":         lrp.Index,
		"domain":        lrp.Domain,
		"instance-guid": lrp.InstanceGuid,
		"cell-id":       lrp.CellId,
		"address":       lrp.Address,
		"ports":         lrp.Ports,
		"evacuating":    evacuating,
		"state":         lrp.State,
	}
}
