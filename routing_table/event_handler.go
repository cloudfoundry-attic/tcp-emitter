package routing_table

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/pivotal-golang/lager"
	"sync"
)

//go:generate counterfeiter -o fakes/fake_event_handler.go . EventHandler
type EventHandler interface {
	HandleEvent(event receptor.Event)
	Sync()
	Syncing() bool
}

type eventHandler struct {
	logger         lager.Logger
	routingTable   RoutingTable
	emitter        Emitter
	receptorClient receptor.Client
	syncing        bool
	cachedEvents   []receptor.Event
	sync.Locker
}

func NewEventHandler(logger lager.Logger, routingTable RoutingTable, emitter Emitter, receptorClient receptor.Client) EventHandler {
	return &eventHandler{
		logger:         logger,
		routingTable:   routingTable,
		emitter:        emitter,
		receptorClient: receptorClient,
		syncing:        false,
		cachedEvents:   nil,
		Locker:         &sync.Mutex{},
	}
}

func (handler *eventHandler) Syncing() bool {
	handler.Lock()
	defer handler.Unlock()
	return handler.syncing
}

func (handler *eventHandler) HandleEvent(event receptor.Event) {
	handler.Lock()
	defer handler.Unlock()
	if handler.syncing {
		handler.logger.Debug("caching-events")
		handler.cachedEvents = append(handler.cachedEvents, event)
	} else {
		handler.handleEvent(event)
	}
}

func (handler *eventHandler) Sync() {
	logger := handler.logger.Session("handle-sync")
	logger.Debug("starting")

	defer func() {
		handler.Lock()
		handler.syncing = false
		handler.cachedEvents = nil
		handler.Unlock()
		logger.Debug("complete")
	}()

	handler.Lock()
	handler.cachedEvents = []receptor.Event{}
	handler.syncing = true
	handler.Unlock()

	var runningActualLRPs []receptor.ActualLRPResponse
	var getActualLRPsErr error
	var desiredLRPs []receptor.DesiredLRPResponse
	var getDesiredLRPsErr error
	var tempRoutingTable RoutingTable

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Debug("getting-actual-lrps")
		actualLRPResponses, err := handler.receptorClient.ActualLRPs()
		if err != nil {
			logger.Error("failed-getting-actual-lrps", err)
			getActualLRPsErr = err
			return
		}
		logger.Debug("succeeded-getting-actual-lrps", lager.Data{"num-actual-responses": len(actualLRPResponses)})

		runningActualLRPs = make([]receptor.ActualLRPResponse, 0, len(actualLRPResponses))
		for _, actualLRPResponse := range actualLRPResponses {
			if actualLRPResponse.State == receptor.ActualLRPStateRunning {
				runningActualLRPs = append(runningActualLRPs, actualLRPResponse)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Debug("getting-desired-lrps")
		desiredLRPResponses, err := handler.receptorClient.DesiredLRPs()
		if err != nil {
			logger.Error("failed-getting-desired-lrps", err)
			getDesiredLRPsErr = err
			return
		}
		logger.Debug("succeeded-getting-desired-lrps", lager.Data{"num-desired-responses": len(desiredLRPResponses)})

		desiredLRPs = make([]receptor.DesiredLRPResponse, 0, len(desiredLRPResponses))
		for _, desiredLRPResponse := range desiredLRPResponses {
			desiredLRPs = append(desiredLRPs, desiredLRPResponse)
		}
	}()

	wg.Wait()

	if getActualLRPsErr == nil && getDesiredLRPsErr == nil {
		tempRoutingTable = NewTable(handler.logger, nil)
		handler.logger.Debug("construct-routing-table")
		for _, desireLrp := range desiredLRPs {
			tempRoutingTable.SetRoutes(desireLrp)
		}

		for _, actualLrp := range runningActualLRPs {
			tempRoutingTable.AddEndpoint(actualLrp)
		}
	} else {
		logger.Info("sync-failed")
	}
	handler.applyCachedEvents(logger, tempRoutingTable)
}

func (handler *eventHandler) applyCachedEvents(logger lager.Logger, tempRoutingTable RoutingTable) {
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

func (handler *eventHandler) handleEvent(event receptor.Event) {
	switch event := event.(type) {
	case receptor.DesiredLRPCreatedEvent:
		handler.handleDesiredCreate(event.DesiredLRPResponse)
	case receptor.DesiredLRPChangedEvent:
		handler.handleDesiredUpdate(event.Before, event.After)
	case receptor.DesiredLRPRemovedEvent:
		handler.handleDesiredDelete(event.DesiredLRPResponse)
	case receptor.ActualLRPCreatedEvent:
		handler.handleActualCreate(event.ActualLRPResponse)
	case receptor.ActualLRPChangedEvent:
		handler.handleActualUpdate(event.Before, event.After)
	case receptor.ActualLRPRemovedEvent:
		handler.handleActualDelete(event.ActualLRPResponse)
	default:
		handler.logger.Info("did-not-handle-unrecognizable-event", lager.Data{"event-type": event.EventType()})
	}
}

func (handler *eventHandler) handleDesiredCreate(desiredLRP receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handle-desired-create", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	routingEvents := handler.routingTable.SetRoutes(desiredLRP)
	handler.emit(routingEvents)
}

func (handler *eventHandler) handleDesiredUpdate(before, after receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handling-desired-update", lager.Data{
		"before": desiredLRPData(before),
		"after":  desiredLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	// TODO: Just call SetRoutes for after...once we have unregistration support we need to add that logic too
	routingEvents := handler.routingTable.SetRoutes(after)
	handler.emit(routingEvents)
}

func (handler *eventHandler) handleDesiredDelete(desiredLRP receptor.DesiredLRPResponse) {
	logger := handler.logger.Session("handling-desired-delete", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	// Do nothing for now...when we support unregistration of routes this needs to do something
}

func (handler *eventHandler) handleActualCreate(actualLRP receptor.ActualLRPResponse) {
	logger := handler.logger.Session("handling-actual-create", actualLRPData(actualLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
	if actualLRP.State == receptor.ActualLRPStateRunning {
		handler.addAndEmit(actualLRP)
	}
}

func (handler *eventHandler) addAndEmit(actualLRP receptor.ActualLRPResponse) {
	routingEvents := handler.routingTable.AddEndpoint(actualLRP)
	handler.emit(routingEvents)
}

func (handler *eventHandler) removeAndEmit(actualLRP receptor.ActualLRPResponse) {
	routingEvents := handler.routingTable.RemoveEndpoint(actualLRP)
	handler.emit(routingEvents)
}

func (handler *eventHandler) emit(routingEvents RoutingEvents) {
	if handler.emitter != nil && len(routingEvents) > 0 {
		handler.emitter.Emit(routingEvents)
	}
}

func (handler *eventHandler) handleActualUpdate(before, after receptor.ActualLRPResponse) {
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

func (handler *eventHandler) handleActualDelete(actualLRP receptor.ActualLRPResponse) {
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
