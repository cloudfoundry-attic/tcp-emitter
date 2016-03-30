package routing_table

import (
	"errors"
	"sync"

	"github.com/cloudfoundry-incubator/bbs"
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_routing_table_handler.go . RoutingTableHandler
type RoutingTableHandler interface {
	HandleEvent(event models.Event)
	Sync()
	Syncing() bool
}

type routingTableHandler struct {
	logger       lager.Logger
	routingTable RoutingTable
	emitter      Emitter
	bbsClient    bbs.Client
	syncing      bool
	cachedEvents []models.Event
	sync.Locker
}

func NewRoutingTableHandler(logger lager.Logger, routingTable RoutingTable, emitter Emitter, bbsClient bbs.Client) RoutingTableHandler {
	return &routingTableHandler{
		logger:       logger,
		routingTable: routingTable,
		emitter:      emitter,
		bbsClient:    bbsClient,
		syncing:      false,
		cachedEvents: nil,
		Locker:       &sync.Mutex{},
	}
}

func (handler *routingTableHandler) Syncing() bool {
	handler.Lock()
	defer handler.Unlock()
	return handler.syncing
}

func (handler *routingTableHandler) HandleEvent(event models.Event) {
	handler.Lock()
	defer handler.Unlock()
	if handler.syncing {
		handler.logger.Debug("caching-events")
		handler.cachedEvents = append(handler.cachedEvents, event)
	} else {
		handler.handleEvent(event)
	}
}

func (handler *routingTableHandler) Sync() {
	logger := handler.logger.Session("bulk-sync")
	logger.Debug("starting")

	var tempRoutingTable RoutingTable

	defer func() {
		handler.Lock()
		handler.applyCachedEvents(logger, tempRoutingTable)
		handler.syncing = false
		handler.cachedEvents = nil
		handler.Unlock()
		logger.Debug("completed")
	}()

	handler.Lock()
	handler.cachedEvents = []models.Event{}
	handler.syncing = true
	handler.Unlock()

	var runningActualLRPs []*models.ActualLRPGroup
	var getActualLRPsErr error
	var desiredLRPs []*models.DesiredLRP
	var getDesiredLRPsErr error

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Debug("getting-actual-lrps")
		actualLRPResponses, err := handler.bbsClient.ActualLRPGroups(models.ActualLRPFilter{})
		if err != nil {
			logger.Error("failed-getting-actual-lrps", err)
			getActualLRPsErr = err
			return
		}
		logger.Debug("succeeded-getting-actual-lrps", lager.Data{"num-actual-responses": len(actualLRPResponses)})

		runningActualLRPs = make([]*models.ActualLRPGroup, 0, len(actualLRPResponses))
		for _, actualLRPResponse := range actualLRPResponses {
			actual, _ := actualLRPResponse.Resolve()
			if actual.State == models.ActualLRPStateRunning {
				runningActualLRPs = append(runningActualLRPs, actualLRPResponse)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Debug("getting-desired-lrps")
		desiredLRPResponses, err := handler.bbsClient.DesiredLRPs(models.DesiredLRPFilter{})
		if err != nil {
			logger.Error("failed-getting-desired-lrps", err)
			getDesiredLRPsErr = err
			return
		}
		logger.Debug("succeeded-getting-desired-lrps", lager.Data{"num-desired-responses": len(desiredLRPResponses)})

		desiredLRPs = make([]*models.DesiredLRP, 0, len(desiredLRPResponses))
		for _, desiredLRPResponse := range desiredLRPResponses {
			desiredLRPs = append(desiredLRPs, desiredLRPResponse)
		}
	}()

	wg.Wait()

	if getActualLRPsErr == nil && getDesiredLRPsErr == nil {
		tempRoutingTable = NewTable(handler.logger, nil)
		handler.logger.Debug("construct-routing-table")
		for _, desireLrp := range desiredLRPs {
			tempRoutingTable.AddRoutes(desireLrp)
		}

		for _, actualLrp := range runningActualLRPs {
			tempRoutingTable.AddEndpoint(actualLrp)
		}
	} else {
		logger.Info("sync-failed")
	}
}

func (handler *routingTableHandler) applyCachedEvents(logger lager.Logger, tempRoutingTable RoutingTable) {
	logger.Debug("apply-cached-events")
	if tempRoutingTable == nil || tempRoutingTable.RouteCount() == 0 {
		// sync failed, process the events on the current table
		handler.logger.Debug("handling-events-from-failed-sync")
		for _, e := range handler.cachedEvents {
			handler.handleEvent(e)
		}
		logger.Debug("done-handling-events-from-failed-sync")
		return
	}

	logger.Debug("tempRoutingTable", lager.Data{"route-count": tempRoutingTable.RouteCount()})

	table := handler.routingTable
	emitter := handler.emitter

	handler.routingTable = tempRoutingTable
	handler.emitter = nil
	for _, e := range handler.cachedEvents {
		handler.handleEvent(e)
	}

	handler.routingTable = table
	handler.emitter = emitter

	logger.Debug("applied-cached-events")
	routingEvents := handler.routingTable.Swap(tempRoutingTable)
	logger.Debug("swap-complete", lager.Data{"events": len(routingEvents)})
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) handleEvent(event models.Event) {
	switch event := event.(type) {
	case *models.DesiredLRPCreatedEvent:
		handler.handleDesiredCreate(event.DesiredLrp)
	case *models.DesiredLRPChangedEvent:
		handler.handleDesiredUpdate(event.Before, event.After)
	case *models.DesiredLRPRemovedEvent:
		handler.handleDesiredDelete(event.DesiredLrp)
	case *models.ActualLRPCreatedEvent:
		handler.handleActualCreate(event.ActualLrpGroup)
	case *models.ActualLRPChangedEvent:
		handler.handleActualUpdate(event.Before, event.After)
	case *models.ActualLRPRemovedEvent:
		handler.handleActualDelete(event.ActualLrpGroup)
	default:
		handler.logger.Error("did-not-handle-unrecognizable-event", errors.New("unrecognizable-event"), lager.Data{"event-type": event.EventType()})
	}
}

func (handler *routingTableHandler) handleDesiredCreate(desiredLRP *models.DesiredLRP) {
	logger := handler.logger.Session("handle-desired-create", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	routingEvents := handler.routingTable.AddRoutes(desiredLRP)
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) handleDesiredUpdate(before, after *models.DesiredLRP) {
	logger := handler.logger.Session("handling-desired-update", lager.Data{
		"before": desiredLRPData(before),
		"after":  desiredLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	routingEvents := handler.routingTable.UpdateRoutes(before, after)
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) handleDesiredDelete(desiredLRP *models.DesiredLRP) {
	logger := handler.logger.Session("handling-desired-delete", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	routingEvents := handler.routingTable.RemoveRoutes(desiredLRP)
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) handleActualCreate(actualLRPGrp *models.ActualLRPGroup) {
	actualLRP, evacuating := actualLRPGrp.Resolve()
	logger := handler.logger.Session("handling-actual-create", actualLRPData(actualLRP, evacuating))
	logger.Debug("starting")
	defer logger.Debug("complete")
	if actualLRP.State == models.ActualLRPStateRunning {
		handler.addAndEmit(actualLRPGrp)
	}
}

func (handler *routingTableHandler) addAndEmit(actualLRPGrp *models.ActualLRPGroup) {
	routingEvents := handler.routingTable.AddEndpoint(actualLRPGrp)
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) removeAndEmit(actualLRPGrp *models.ActualLRPGroup) {
	routingEvents := handler.routingTable.RemoveEndpoint(actualLRPGrp)
	handler.emit(routingEvents)
}

func (handler *routingTableHandler) emit(routingEvents RoutingEvents) {
	if handler.emitter != nil && len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *routingTableHandler) handleActualUpdate(beforeGrp, afterGrp *models.ActualLRPGroup) {
	before, beforeEvacuating := beforeGrp.Resolve()
	after, afterEvacuating := afterGrp.Resolve()
	logger := handler.logger.Session("handling-actual-update", lager.Data{
		"before": actualLRPData(before, beforeEvacuating),
		"after":  actualLRPData(after, afterEvacuating),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	switch {
	case after.State == models.ActualLRPStateRunning:
		handler.addAndEmit(afterGrp)
	case after.State != models.ActualLRPStateRunning && before.State == models.ActualLRPStateRunning:
		handler.removeAndEmit(beforeGrp)
	}
}

func (handler *routingTableHandler) handleActualDelete(actualLRPGrp *models.ActualLRPGroup) {
	actualLRP, evacuating := actualLRPGrp.Resolve()
	logger := handler.logger.Session("handling-actual-delete", actualLRPData(actualLRP, evacuating))
	logger.Debug("starting")
	defer logger.Debug("complete")
	if actualLRP.State == models.ActualLRPStateRunning {
		handler.removeAndEmit(actualLRPGrp)
	}
}

func desiredLRPData(lrp *models.DesiredLRP) lager.Data {
	return lager.Data{
		"process-guid": lrp.ProcessGuid,
		"routes":       lrp.Routes,
		"ports":        lrp.Ports,
	}
}

func actualLRPData(lrp *models.ActualLRP, evacuating bool) lager.Data {
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
