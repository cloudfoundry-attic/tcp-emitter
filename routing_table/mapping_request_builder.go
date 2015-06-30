package routing_table

import (
	cf_tcp_router "github.com/cloudfoundry-incubator/cf-tcp-router"
)

func BuildMappingRequests(routingEvents RoutingEvents) cf_tcp_router.MappingRequests {
	mappingRequests := cf_tcp_router.MappingRequests{}
	for _, routingEvent := range routingEvents {
		mappingRequest := mapRoutingEvent(routingEvent)
		if mappingRequest != nil {
			mappingRequests = append(mappingRequests, *mappingRequest)
		}
	}
	return mappingRequests
}

func mapRoutingEvent(routingEvent RoutingEvent) *cf_tcp_router.MappingRequest {
	if routingEvent.Entry.ExternalEndpoint.Port == 0 {
		return nil
	}
	if len(routingEvent.Entry.Endpoints) == 0 {
		return nil
	}
	backends := cf_tcp_router.BackendHostInfos{}
	for _, endpoint := range routingEvent.Entry.Endpoints {
		backends = append(backends, cf_tcp_router.NewBackendHostInfo(endpoint.Host, endpoint.Port))
	}
	mappingRequest := cf_tcp_router.NewMappingRequest(routingEvent.Entry.ExternalEndpoint.Port, backends)
	return &mappingRequest
}
