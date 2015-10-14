package routing_table

import (
	"errors"

	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/pivotal-golang/lager"
)

func buildMappingRequests(routingEvents RoutingEvents) []db.TcpRouteMapping {
	mappingRequests := make([]db.TcpRouteMapping, 0)
	for _, routingEvent := range routingEvents {
		mappingRequest := mapRoutingEvent(routingEvent)
		if mappingRequest != nil {
			mappingRequests = append(mappingRequests, (*mappingRequest)...)
		}
	}
	return mappingRequests
}

func mapRoutingEvent(routingEvent RoutingEvent) *[]db.TcpRouteMapping {
	mappingRequests := make([]db.TcpRouteMapping, 0)
	for _, externalEndpoint := range routingEvent.Entry.ExternalEndpoints {
		for _, endpoint := range routingEvent.Entry.Endpoints {
			mappingRequests = append(mappingRequests, db.NewTcpRouteMapping(externalEndpoint.RouterGroupGuid, uint16(externalEndpoint.Port),
				endpoint.Host, uint16(endpoint.Port)))
		}
	}
	return &mappingRequests
}

func CreateMappingRequests(logger lager.Logger, routingEvents RoutingEvents) ([]db.TcpRouteMapping, []db.TcpRouteMapping) {
	registrationEvents := RoutingEvents{}
	unregistrationEvents := RoutingEvents{}
	for _, routingEvent := range routingEvents {
		if !routingEvent.Valid() {
			logger.Error("invalid-routing-event", errors.New("Invalid routing event"), lager.Data{"routing-event-key": routingEvent.Key})
			continue
		}

		if routingEvent.EventType == RouteRegistrationEvent {
			registrationEvents = append(registrationEvents, routingEvent)
		} else if routingEvent.EventType == RouteUnregistrationEvent {
			unregistrationEvents = append(unregistrationEvents, routingEvent)
		}
	}

	registrationMappingRequests := buildMappingRequests(registrationEvents)

	unregistrationMappingRequests := buildMappingRequests(unregistrationEvents)

	return registrationMappingRequests, unregistrationMappingRequests
}
