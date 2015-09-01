package routing_table

import (
	"sync"

	"github.com/cloudfoundry-incubator/receptor"

	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_routing_table.go . RoutingTable
type RoutingTable interface {
	RouteCount() int

	SetRoutes(desiredLRP receptor.DesiredLRPResponse) RoutingEvents

	AddEndpoint(actualLRP receptor.ActualLRPResponse) RoutingEvents
	RemoveEndpoint(actualLRP receptor.ActualLRPResponse) RoutingEvents

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
		routingEvents = append(routingEvents, table.desiredLRPRegistrationEvents(table.logger, key, entry)...)
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
		routingEvents = append(routingEvents, table.desiredLRPRegistrationEvents(table.logger, key, newEntry)...)
	}
	table.entries = newEntries

	//TODO: We need to go over existing entries and generate unregistration messages

	return routingEvents
}

func (table *routingTable) RouteCount() int {
	table.Lock()
	defer table.Unlock()
	return len(table.entries)
}

func (table *routingTable) SetRoutes(desiredLRP receptor.DesiredLRPResponse) RoutingEvents {
	logger := table.logger.Session("SetRoutes", lager.Data{"desired_lrp": desiredLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	routingKeys := RoutingKeysFromDesired(desiredLRP)
	routes, _ := tcp_routes.TCPRoutesFromRoutingInfo(desiredLRP.Routes)

	table.Lock()
	defer table.Unlock()

	updatedKeys := make(map[RoutingKey]struct{})
	for _, key := range routingKeys {
		existingEntry := table.entries[key]
		existingModificationTag := existingEntry.ModificationTag
		if !existingModificationTag.SucceededBy(desiredLRP.ModificationTag) {
			continue
		}

		if table.setRoutes(existingEntry, routes, key) {
			logger.Debug("change-to-external-ports", lager.Data{"key": key})
			updatedKeys[key] = struct{}{}
		}
	}

	return table.generateRoutingEvents(logger, updatedKeys, desiredLRP.LogGuid, desiredLRP.ModificationTag)
}

func (table *routingTable) setRoutes(
	existingEntry RoutableEndpoints,
	routes tcp_routes.TCPRoutes,
	key RoutingKey) bool {
	var updated bool

	for _, route := range routes {
		if key.ContainerPort == route.ContainerPort &&
			isNewExternalEndpoint(existingEntry.ExternalEndpoints, route.ExternalPort) {
			existingEntry.ExternalEndpoints = append(existingEntry.ExternalEndpoints,
				NewExternalEndpointInfo(route.ExternalPort))

			table.entries[key] = existingEntry
			updated = true
		}
	}

	return updated
}

func (table *routingTable) generateRoutingEvents(logger lager.Logger,
	updatedKeys map[RoutingKey]struct{}, logGuid string, modificationTag receptor.ModificationTag) RoutingEvents {

	routingEvents := RoutingEvents{}
	for key, _ := range updatedKeys {
		existingEntry := table.entries[key]
		existingEntry.LogGuid = logGuid
		existingEntry.ModificationTag = modificationTag
		routingEvents = append(routingEvents, table.desiredLRPRegistrationEvents(logger, key, existingEntry)...)
		table.entries[key] = existingEntry
	}
	return routingEvents
}

func (table *routingTable) AddEndpoint(actualLRP receptor.ActualLRPResponse) RoutingEvents {
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

func (table *routingTable) RemoveEndpoint(actualLRP receptor.ActualLRPResponse) RoutingEvents {
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

	if !ok || !(currentEndpoint.ModificationTag.Equal(endpoint.ModificationTag) || currentEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag)) {
		return RoutingEvents{}
	}

	newEntry := currentEntry.copy()
	delete(newEntry.Endpoints, endpointKey)
	table.entries[key] = newEntry

	// Once tcp router supports unregistration we need to change this to send unregistration events
	// So for  now if number of endpoints go down to zero we would see no events being sent....
	return table.getRegistrationEvents(logger, key, currentEntry, newEntry)
}

func (table *routingTable) getRegistrationEvents(logger lager.Logger, key RoutingKey, existingEntry, newEntry RoutableEndpoints) RoutingEvents {
	logger.Debug("get-registration-events")
	if hasNoExternalPorts(logger, newEntry.ExternalEndpoints) {
		return RoutingEvents{}
	}

	if !haveExternalEndpointsChanged(existingEntry, newEntry) &&
		!haveEndpointsChanged(existingEntry, newEntry) {
		logger.Debug("no-change-to-endpoints")
		return RoutingEvents{}
	}

	// We are replacing the whole mapping so just check if there exists any endpoints
	if len(newEntry.Endpoints) > 0 {
		return RoutingEvents{
			RoutingEvent{
				EventType: RouteRegistrationEvent,
				Key:       key,
				Entry:     newEntry,
			},
		}
	}
	return RoutingEvents{}
}

func (table *routingTable) desiredLRPRegistrationEvents(logger lager.Logger, key RoutingKey, entry RoutableEndpoints) RoutingEvents {
	logger.Debug("get-registration-events")
	if hasNoExternalPorts(logger, entry.ExternalEndpoints) {
		return RoutingEvents{}
	}

	// We are replacing the whole mapping so just check if there exists any endpoints
	if len(entry.Endpoints) > 0 {
		logger.Debug("endpoints", lager.Data{"count": len(entry.Endpoints)})
		return RoutingEvents{
			RoutingEvent{
				EventType: RouteRegistrationEvent,
				Key:       key,
				Entry:     entry,
			},
		}
	}
	return RoutingEvents{}
}

func hasNoExternalPorts(logger lager.Logger, externalEndpoints ExternalEndpointInfos) bool {
	if externalEndpoints == nil || len(externalEndpoints) == 0 {
		logger.Debug("no-external-port")
		return true
	}
	// This originally checked if Port was 0, I think to see if it was a zero value, check and make sure
	return false
}

func isNewExternalEndpoint(endpoints ExternalEndpointInfos, port uint16) bool {
	for _, existing := range endpoints {
		if existing.Port == port {
			return false
		}
	}
	return true
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
