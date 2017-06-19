package util

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
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

func DesiredLRPSchedulingInfoData(lrpInfo *models.DesiredLRPSchedulingInfo) lager.Data {
	logRoutes := make(models.Routes)
	logRoutes[cfroutes.CF_ROUTER] = (lrpInfo.Routes)[cfroutes.CF_ROUTER]
	logRoutes[tcp_routes.TCP_ROUTER] = (lrpInfo.Routes)[tcp_routes.TCP_ROUTER]

	return lager.Data{
		"process-guid": lrpInfo.ProcessGuid,
		"routes":       logRoutes,
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

func DesiredLRPSchedulingInfo(desiredLRP *models.DesiredLRP) *models.DesiredLRPSchedulingInfo {
	return &models.DesiredLRPSchedulingInfo{
		DesiredLRPKey: models.DesiredLRPKey{
			ProcessGuid: desiredLRP.ProcessGuid,
			LogGuid:     desiredLRP.LogGuid,
		},
		Routes:          *desiredLRP.Routes,
		ModificationTag: *desiredLRP.ModificationTag,
	}
}
