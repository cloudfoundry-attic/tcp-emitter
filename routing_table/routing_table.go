package routing_table

import (
	"encoding/json"
	"sync"

	"github.com/cloudfoundry-incubator/bbs/models"

	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	"github.com/pivotal-golang/lager"
)

type routeInfo struct {
	ProcessGuid string
	Routes      map[string]*json.RawMessage
}

//go:generate counterfeiter -o fakes/fake_routing_table.go . RoutingTable
type RoutingTable interface {
	RouteCount() int

	AddRoutes(desiredLRP *models.DesiredLRP) RoutingEvents
	UpdateRoutes(beforeLRP, afterLRP *models.DesiredLRP) RoutingEvents
	RemoveRoutes(desiredLRP *models.DesiredLRP) RoutingEvents

	AddEndpoint(actualLRP *models.ActualLRPGroup) RoutingEvents
	RemoveEndpoint(actualLRP *models.ActualLRPGroup) RoutingEvents

	Swap(t RoutingTable) RoutingEvents

	GetRoutingEvents() RoutingEvents
}

type routingTable struct {
	entries map[RoutingKey]RoutableEndpoints
	sync.Locker
	logger lager.Logger
}

func NewTable(logger lager.Logger, entries map[RoutingKey]RoutableEndpoints) RoutingTable {
	if entries == nil {
		entries = make(map[RoutingKey]RoutableEndpoints)
	}
	return &routingTable{
		entries: entries,
		Locker:  &sync.Mutex{},
		logger:  logger,
	}
}

func (table *routingTable) GetRoutingEvents() RoutingEvents {
	routingEvents := RoutingEvents{}

	table.Lock()
	defer table.Unlock()
	table.logger.Debug("get-routing-events", lager.Data{"count": len(table.entries)})

	for key, entry := range table.entries {
		//always register everything on sync
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, entry, RouteRegistrationEvent)...)
	}

	return routingEvents
}

func (table *routingTable) Swap(t RoutingTable) RoutingEvents {

	routingEvents := RoutingEvents{}

	newTable, ok := t.(*routingTable)
	if !ok {
		return routingEvents
	}

	table.Lock()
	defer table.Unlock()

	newEntries := newTable.entries
	for key, newEntry := range newEntries {
		//always register everything on sync
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, newEntry, RouteRegistrationEvent)...)

		newExternalEndpoints := newEntry.ExternalEndpoints
		existingEntry := table.entries[key]

		unregistrationEntry := existingEntry.RemoveExternalEndpoints(newExternalEndpoints)
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, unregistrationEntry, RouteUnregistrationEvent)...)
	}

	for key, existingEntry := range table.entries {
		if _, ok := newEntries[key]; !ok {
			routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, existingEntry, RouteUnregistrationEvent)...)
		}
	}

	table.entries = newEntries

	return routingEvents
}

func (table *routingTable) RouteCount() int {
	table.Lock()
	defer table.Unlock()
	return len(table.entries)
}

func (table *routingTable) AddRoutes(desiredLRP *models.DesiredLRP) RoutingEvents {
	logger := table.logger.Session("AddRoutes", lager.Data{"desired_lrp": desiredLRPData(desiredLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	table.Lock()
	defer table.Unlock()

	return table.addRoutes(logger, desiredLRP)
}

func (table *routingTable) addRoutes(logger lager.Logger, desiredLRP *models.DesiredLRP) RoutingEvents {
	routingKeys := RoutingKeysFromDesired(desiredLRP)
	routes, _ := tcp_routes.TCPRoutesFromRoutingInfo(desiredLRP.Routes)

	routingEvents := RoutingEvents{}
	for _, key := range routingKeys {
		existingEntry := table.entries[key]
		existingModificationTag := existingEntry.ModificationTag
		if !existingModificationTag.SucceededBy(desiredLRP.ModificationTag) {
			continue
		}
		routingEvents = append(routingEvents, table.mergeRoutes(logger, existingEntry,
			routes, key, desiredLRP.LogGuid, desiredLRP.ModificationTag)...)
	}
	return routingEvents
}

func (table *routingTable) UpdateRoutes(beforeLRP, afterLRP *models.DesiredLRP) RoutingEvents {
	logger := table.logger.Session("UpdateRoutes", lager.Data{"before_lrp": desiredLRPData(beforeLRP), "after_lrp": desiredLRPData(afterLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	beforeRoutingKeys := RoutingKeysFromDesired(beforeLRP)
	afterRoutingKeys := RoutingKeysFromDesired(afterLRP)

	deletedRoutingKeys := beforeRoutingKeys.Remove(afterRoutingKeys)
	logger.Debug("keys-to-be-deleted", lager.Data{"count": len(deletedRoutingKeys)})

	table.Lock()
	defer table.Unlock()

	routingEvents := table.addRoutes(logger, afterLRP)
	routingEvents = append(routingEvents,
		table.removeRoutingKeys(logger, deletedRoutingKeys, afterLRP.ModificationTag)...)
	return routingEvents
}

func (table *routingTable) RemoveRoutes(desiredLRP *models.DesiredLRP) RoutingEvents {
	logger := table.logger.Session("RemoveRoutes", lager.Data{"desired_lrp": desiredLRPData(desiredLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	routingKeys := RoutingKeysFromDesired(desiredLRP)

	table.Lock()
	defer table.Unlock()

	routingEvents := table.removeRoutingKeys(logger, routingKeys, desiredLRP.ModificationTag)
	return routingEvents
}

func (table *routingTable) removeRoutingKeys(logger lager.Logger, routingKeys RoutingKeys,
	modificationTag *models.ModificationTag) RoutingEvents {
	routingEvents := RoutingEvents{}
	for _, key := range routingKeys {
		if existingEntry, ok := table.entries[key]; ok {
			existingModificationTag := existingEntry.ModificationTag
			if !existingModificationTag.SucceededBy(modificationTag) {
				continue
			}
			if len(existingEntry.Endpoints) > 0 {
				routingEvents = append(routingEvents, RoutingEvent{
					EventType: RouteUnregistrationEvent,
					Key:       key,
					Entry:     existingEntry,
				})
			}

			delete(table.entries, key)
			logger.Debug("route-deleted", lager.Data{"routing-key": key})
		}
	}
	return routingEvents
}

func (table *routingTable) mergeRoutes(
	logger lager.Logger,
	existingEntry RoutableEndpoints,
	routes tcp_routes.TCPRoutes,
	key RoutingKey,
	logGUID string,
	modificationTag *models.ModificationTag) RoutingEvents {
	var registrationNeeded bool

	var newExternalEndpoints ExternalEndpointInfos

	for _, route := range routes {
		if key.ContainerPort == route.ContainerPort {
			if !containsExternalPort(existingEntry.ExternalEndpoints, route.ExternalPort) {
				newExternalEndpoints = append(newExternalEndpoints,
					NewExternalEndpointInfo(route.RouterGroupGuid, route.ExternalPort))
				registrationNeeded = true
			} else {
				newExternalEndpoints = append(newExternalEndpoints,
					NewExternalEndpointInfo(route.RouterGroupGuid, route.ExternalPort))
			}
		}
	}

	routingEvents := RoutingEvents{}

	if registrationNeeded {
		updatedEntry := existingEntry.copy()
		updatedEntry.ExternalEndpoints = newExternalEndpoints
		updatedEntry.LogGUID = logGUID
		updatedEntry.ModificationTag = modificationTag
		table.entries[key] = updatedEntry
		routingEvents = append(routingEvents, table.createRoutingEvent(logger, key, updatedEntry, RouteRegistrationEvent)...)
		logger.Debug("routing-table-entry-updated", lager.Data{"key": key})
	}

	unregistrationEntry := existingEntry.RemoveExternalEndpoints(newExternalEndpoints)
	routingEvents = append(routingEvents, table.createRoutingEvent(logger, key, unregistrationEntry, RouteUnregistrationEvent)...)

	return routingEvents
}

func (table *routingTable) AddEndpoint(actualLRP *models.ActualLRPGroup) RoutingEvents {
	logger := table.logger.Session("AddEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	endpoints, _ := EndpointsFromActual(actualLRP)

	routingEvents := RoutingEvents{}

	for _, key := range RoutingKeysFromActual(actualLRP) {
		for _, endpoint := range endpoints {
			if key.ContainerPort == endpoint.ContainerPort {
				routingEvents = append(routingEvents, table.addEndpoint(logger, key, endpoint)...)
			}
		}
	}
	return routingEvents
}

func (table *routingTable) addEndpoint(logger lager.Logger, key RoutingKey, endpoint Endpoint) RoutingEvents {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]

	if existingEndpoint, ok := currentEntry.Endpoints[endpoint.key()]; ok {
		if !existingEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag) {
			return RoutingEvents{}
		}
	}

	newEntry := currentEntry.copy()
	newEntry.Endpoints[endpoint.key()] = endpoint
	table.entries[key] = newEntry

	return table.getRegistrationEvents(logger, key, currentEntry, newEntry)
}

func (table *routingTable) RemoveEndpoint(actualLRP *models.ActualLRPGroup) RoutingEvents {
	logger := table.logger.Session("RemoveEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	endpoints, _ := EndpointsFromActual(actualLRP)

	routingEvents := RoutingEvents{}

	for _, key := range RoutingKeysFromActual(actualLRP) {
		for _, endpoint := range endpoints {
			if key.ContainerPort == endpoint.ContainerPort {
				routingEvents = append(routingEvents, table.removeEndpoint(logger, key, endpoint)...)
			}
		}
	}
	return routingEvents
}

func (table *routingTable) removeEndpoint(logger lager.Logger, key RoutingKey, endpoint Endpoint) RoutingEvents {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	endpointKey := endpoint.key()
	currentEndpoint, ok := currentEntry.Endpoints[endpointKey]

	if !ok || !(currentEndpoint.ModificationTag.Equal(endpoint.ModificationTag) ||
		currentEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag)) {
		return RoutingEvents{}
	}

	newEntry := currentEntry.copy()
	delete(newEntry.Endpoints, endpointKey)
	table.entries[key] = newEntry

	if !haveExternalEndpointsChanged(currentEntry, newEntry) && !haveEndpointsChanged(currentEntry, newEntry) {
		logger.Debug("no-change-to-endpoints")
		return RoutingEvents{}
	}

	deletedEntry := table.getDeletedEntry(currentEntry, newEntry)

	return table.createRoutingEvent(logger, key, deletedEntry, RouteUnregistrationEvent)
}

func (table *routingTable) getRegistrationEvents(
	logger lager.Logger,
	key RoutingKey,
	existingEntry, newEntry RoutableEndpoints) RoutingEvents {
	logger.Debug("get-registration-events")
	if hasNoExternalPorts(logger, newEntry.ExternalEndpoints) {
		return RoutingEvents{}
	}

	if !haveExternalEndpointsChanged(existingEntry, newEntry) &&
		!haveEndpointsChanged(existingEntry, newEntry) {
		logger.Debug("no-change-to-endpoints")
		return RoutingEvents{}
	}

	routingEvents := RoutingEvents{}

	// We are replacing the whole mapping so just check if there exists any endpoints
	if len(newEntry.Endpoints) > 0 {
		routingEvents = append(routingEvents, RoutingEvent{
			EventType: RouteRegistrationEvent,
			Key:       key,
			Entry:     newEntry,
		})
	}
	return routingEvents
}

func (table *routingTable) createRoutingEvent(logger lager.Logger, key RoutingKey, entry RoutableEndpoints, eventType RoutingEventType) RoutingEvents {
	logger.Debug("create-routing-events")
	// in which case does a entry end up with no external endpoints ?
	if hasNoExternalPorts(logger, entry.ExternalEndpoints) {
		return RoutingEvents{}
	}

	if len(entry.Endpoints) > 0 {
		logger.Debug("endpoints", lager.Data{"count": len(entry.Endpoints)})
		return RoutingEvents{
			RoutingEvent{
				EventType: eventType,
				Key:       key,
				Entry:     entry,
			},
		}
	}
	return RoutingEvents{}
}

func (table *routingTable) getDeletedEntry(existingEntry, newEntry RoutableEndpoints) RoutableEndpoints {
	// Assuming ExternalEndpoints for both existingEntry, newEntry are the same.
	gapEntry := existingEntry.copy()
	for endpointKey := range existingEntry.Endpoints {
		if _, ok := newEntry.Endpoints[endpointKey]; ok {
			delete(gapEntry.Endpoints, endpointKey)
		}
	}
	return gapEntry
}

func hasNoExternalPorts(logger lager.Logger, externalEndpoints ExternalEndpointInfos) bool {
	if externalEndpoints == nil || len(externalEndpoints) == 0 {
		logger.Debug("no-external-port")
		return true
	}
	// This originally checked if Port was 0, I think to see if it was a zero value, check and make sure
	return false
}

func containsExternalPort(endpoints ExternalEndpointInfos, port uint32) bool {
	for _, existing := range endpoints {
		if existing.Port == port {
			return true
		}
	}
	return false
}

func haveExternalEndpointsChanged(existingEntry, newEntry RoutableEndpoints) bool {
	if len(existingEntry.ExternalEndpoints) != len(newEntry.ExternalEndpoints) {
		// length not same...so something changed
		return true
	}
	//Check if new endpoints are added
	for _, existing := range existingEntry.ExternalEndpoints {
		found := false
		for _, proposed := range newEntry.ExternalEndpoints {
			if proposed.Port == existing.Port {
				found = true
				break
			}
		}

		// Could not find existing endpoint, something changed
		if !found {
			return true
		}
	}
	return false
}

func haveEndpointsChanged(existingEntry, newEntry RoutableEndpoints) bool {
	if len(existingEntry.Endpoints) != len(newEntry.Endpoints) {
		// length not same...so something changed
		return true
	}
	//Check if new endpoints are added or existing endpoints are modified
	for key, newEndpoint := range newEntry.Endpoints {
		if existingEndpoint, ok := existingEntry.Endpoints[key]; !ok {
			// new endpoint
			return true
		} else {
			if existingEndpoint.ModificationTag.SucceededBy(newEndpoint.ModificationTag) {
				// existing endpoint modified
				return true
			}
		}
	}
	return false
}
