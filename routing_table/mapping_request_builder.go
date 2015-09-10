package routing_table

import (
	cf_tcp_router "github.com/cloudfoundry-incubator/cf-tcp-router"
)

func BuildMappingRequests(routingEvents RoutingEvents) cf_tcp_router.MappingRequests {
	mappingRequests := cf_tcp_router.MappingRequests{}
	for _, routingEvent := range routingEvents {
		mappingRequest := mapRoutingEvent(routingEvent)
		if mappingRequest != nil {
			mappingRequests = append(mappingRequests, (*mappingRequest)...)
		}
	}
	return mappingRequests
}

func mapRoutingEvent(routingEvent RoutingEvent) *cf_tcp_router.MappingRequests {
	if len(routingEvent.Entry.Endpoints) == 0 {
		return nil
	}

	mappingRequests := cf_tcp_router.MappingRequests{}
	for _, externalEndpoint := range routingEvent.Entry.ExternalEndpoints {
		if externalEndpoint.Port == 0 {
			continue
		}

		backends := cf_tcp_router.BackendHostInfos{}
		for _, endpoint := range routingEvent.Entry.Endpoints {
			backends = append(backends, cf_tcp_router.NewBackendHostInfo(endpoint.Host, uint16(endpoint.Port)))
		}
		mappingRequests = append(mappingRequests, cf_tcp_router.NewMappingRequest(uint16(externalEndpoint.Port), backends))
	}
	return &mappingRequests
}
