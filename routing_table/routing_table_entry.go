package routing_table

import "github.com/cloudfoundry-incubator/receptor"

type EndpointKey struct {
	InstanceGuid string
	Evacuating   bool
}

func NewEndpointKey(instanceGuid string, evacuating bool) EndpointKey {
	return EndpointKey{
		InstanceGuid: instanceGuid,
		Evacuating:   evacuating,
	}
}

type Endpoint struct {
	InstanceGuid    string
	Host            string
	Port            uint16
	ContainerPort   uint16
	Evacuating      bool
	ModificationTag receptor.ModificationTag
}

func (e Endpoint) key() EndpointKey {
	return EndpointKey{InstanceGuid: e.InstanceGuid, Evacuating: e.Evacuating}
}

func NewEndpoint(
	instanceGuid string, evacuating bool,
	host string, port, containerPort uint16,
	modificationTag receptor.ModificationTag) Endpoint {
	return Endpoint{
		InstanceGuid:    instanceGuid,
		Evacuating:      evacuating,
		Host:            host,
		Port:            port,
		ContainerPort:   containerPort,
		ModificationTag: modificationTag,
	}
}

type ExternalEndpointInfo struct {
	Port uint16
}

func NewExternalEndpointInfo(port uint16) ExternalEndpointInfo {
	return ExternalEndpointInfo{
		Port: port,
	}
}

type RoutableEndpoints struct {
	ExternalEndpoint ExternalEndpointInfo
	Endpoints        map[EndpointKey]Endpoint
	LogGuid          string
	ModificationTag  receptor.ModificationTag
}

func (entry RoutableEndpoints) copy() RoutableEndpoints {
	clone := RoutableEndpoints{
		ExternalEndpoint: entry.ExternalEndpoint,
		Endpoints:        map[EndpointKey]Endpoint{},
		LogGuid:          entry.LogGuid,
		ModificationTag:  entry.ModificationTag,
	}

	for k, v := range entry.Endpoints {
		clone.Endpoints[k] = v
	}

	return clone
}

func NewRoutableEndpoints(
	externalEndPoint ExternalEndpointInfo,
	endpoints map[EndpointKey]Endpoint,
	logGuid string,
	modificationTag receptor.ModificationTag) RoutableEndpoints {
	return RoutableEndpoints{
		ExternalEndpoint: externalEndPoint,
		Endpoints:        endpoints,
		LogGuid:          logGuid,
		ModificationTag:  modificationTag,
	}
}

type RoutingKey struct {
	ProcessGuid   string
	ContainerPort uint16
}

func NewRoutingKey(processGuid string, containerPort uint16) RoutingKey {
	return RoutingKey{
		ProcessGuid:   processGuid,
		ContainerPort: containerPort,
	}
}
