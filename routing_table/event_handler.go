package routing_table

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_event_handler.go . EventHandler
type EventHandler interface {
	HandleDesiredCreate(desiredLRP receptor.DesiredLRPResponse)
	HandleDesiredUpdate(before, after receptor.DesiredLRPResponse)
	HandleDesiredDelete(desiredLRP receptor.DesiredLRPResponse)
	HandleActualCreate(actualLRP receptor.ActualLRPResponse)
	HandleActualUpdate(before, after receptor.ActualLRPResponse)
	HandleActualDelete(actualLRP receptor.ActualLRPResponse)
}

type eventHandler struct {
	logger       lager.Logger
	routingTable RoutingTable
	emitter      Emitter
}

func NewEventHandler(logger lager.Logger, routingTable RoutingTable, emitter Emitter) EventHandler {
	return &eventHandler{
		logger:       logger,
		routingTable: routingTable,
		emitter:      emitter,
	}
}

func (handler *eventHandler) HandleDesiredCreate(desiredLRP receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handle-desired-create", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	routingEvents := handler.routingTable.SetRoutes(desiredLRP)
	if len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *eventHandler) HandleDesiredUpdate(before, after receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handling-desired-update", lager.Data{
		"before": desiredLRPData(before),
		"after":  desiredLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	// Just call SetRoutes for after...once we have unregistration support we need to add that logic too
	routingEvents := handler.routingTable.SetRoutes(after)
	if len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *eventHandler) HandleDesiredDelete(desiredLRP receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handling-desired-delete", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	// Do nothing for now...when we support unregistration of routes this needs to do something
}

func (handler *eventHandler) HandleActualCreate(actualLRP receptor.ActualLRPResponse) {
	logger := handler.logger.Session("handling-actual-create", actualLRPData(actualLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	if actualLRP.State == receptor.ActualLRPStateRunning {
		handler.addAndEmit(actualLRP)
	}
}

func (handler *eventHandler) addAndEmit(actualLRP receptor.ActualLRPResponse) {
	routingEvents := handler.routingTable.AddEndpoint(actualLRP)
	if len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *eventHandler) removeAndEmit(actualLRP receptor.ActualLRPResponse) {
	routingEvents := handler.routingTable.RemoveEndpoint(actualLRP)
	if len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *eventHandler) HandleActualUpdate(before, after receptor.ActualLRPResponse) {
	logger := handler.logger.Session("handling-actual-update", lager.Data{
		"before": actualLRPData(before),
		"after":  actualLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	switch {
	case after.State == receptor.ActualLRPStateRunning:
		handler.addAndEmit(after)
	case after.State != receptor.ActualLRPStateRunning && before.State == receptor.ActualLRPStateRunning:
		handler.removeAndEmit(before)
	}
}

func (handler *eventHandler) HandleActualDelete(actualLRP receptor.ActualLRPResponse) {
	logger := handler.logger.Session("handling-actual-delete", actualLRPData(actualLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	if actualLRP.State == receptor.ActualLRPStateRunning {
		handler.removeAndEmit(actualLRP)
	}
}

func desiredLRPData(lrp receptor.DesiredLRPResponse) lager.Data {
	return lager.Data{
		"process-guid": lrp.ProcessGuid,
		"routes":       lrp.Routes,
		"ports":        lrp.Ports,
	}
}

func actualLRPData(lrp receptor.ActualLRPResponse) lager.Data {
	return lager.Data{
		"process-guid":  lrp.ProcessGuid,
		"index":         lrp.Index,
		"domain":        lrp.Domain,
		"instance-guid": lrp.InstanceGuid,
		"cell-id":       lrp.CellID,
		"address":       lrp.Address,
		"ports":         lrp.Ports,
		"evacuating":    lrp.Evacuating,
		"state":         lrp.State,
	}
}
