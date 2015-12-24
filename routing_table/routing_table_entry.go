package routing_table

import "github.com/cloudfoundry-incubator/bbs/models"

type EndpointKey struct {
	InstanceGUID string
	Evacuating   bool
}

func NewEndpointKey(instanceGUID string, evacuating bool) EndpointKey {
	return EndpointKey{
		InstanceGUID: instanceGUID,
		Evacuating:   evacuating,
	}
}

type Endpoint struct {
	InstanceGUID    string
	Host            string
	Port            uint32
	ContainerPort   uint32
	Evacuating      bool
	ModificationTag *models.ModificationTag
}

func (e Endpoint) key() EndpointKey {
	return EndpointKey{InstanceGUID: e.InstanceGUID, Evacuating: e.Evacuating}
}

func NewEndpoint(
	instanceGUID string, evacuating bool,
	host string, port, containerPort uint32,
	modificationTag *models.ModificationTag) Endpoint {
	return Endpoint{
		InstanceGUID:    instanceGUID,
		Evacuating:      evacuating,
		Host:            host,
		Port:            port,
		ContainerPort:   containerPort,
		ModificationTag: modificationTag,
	}
}

type ExternalEndpointInfo struct {
	RouterGroupGUID string
	Port            uint32
}

type ExternalEndpointInfos []ExternalEndpointInfo

func NewExternalEndpointInfo(routerGroupGUID string, port uint32) ExternalEndpointInfo {
	return ExternalEndpointInfo{
		RouterGroupGUID: routerGroupGUID,
		Port:            port,
	}
}

type RoutableEndpoints struct {
	ExternalEndpoints ExternalEndpointInfos
	Endpoints         map[EndpointKey]Endpoint
	LogGUID           string
	ModificationTag   *models.ModificationTag
}

func (entry RoutableEndpoints) copy() RoutableEndpoints {
	clone := RoutableEndpoints{
		ExternalEndpoints: entry.ExternalEndpoints,
		Endpoints:         map[EndpointKey]Endpoint{},
		LogGUID:           entry.LogGUID,
		ModificationTag:   entry.ModificationTag,
	}

	for k, v := range entry.Endpoints {
		clone.Endpoints[k] = v
	}

	return clone
}

func NewRoutableEndpoints(
	externalEndPoint ExternalEndpointInfos,
	endpoints map[EndpointKey]Endpoint,
	logGUID string,
	modificationTag *models.ModificationTag) RoutableEndpoints {
	return RoutableEndpoints{
		ExternalEndpoints: externalEndPoint,
		Endpoints:         endpoints,
		LogGUID:           logGUID,
		ModificationTag:   modificationTag,
	}
}

type RoutingKeys []RoutingKey

type RoutingKey struct {
	ProcessGUID   string
	ContainerPort uint32
}

func NewRoutingKey(processGUID string, containerPort uint32) RoutingKey {
	return RoutingKey{
		ProcessGUID:   processGUID,
		ContainerPort: containerPort,
	}
}

// this function returns the entry with the external externalEndpoints substracted from its internal collection
// Ex; Given, entry { externalEndpoints={p1,p2,p4} } and externalEndpoints = {p2,p3} ==> entryA { externalEndpoints={p1,p4} }
func (entry RoutableEndpoints) RemoveExternalEndpoints(externalEndpoints ExternalEndpointInfos) RoutableEndpoints {
	subtractedExternalEndpoints := entry.ExternalEndpoints.Remove(externalEndpoints)
	resultEntry := entry.copy()
	resultEntry.ExternalEndpoints = subtractedExternalEndpoints
	return resultEntry
}

// this function return a-b set. Ex: a = {p1,p2, p4} b={p2,p3} ===> a-b = {p1, p4}
func (setA ExternalEndpointInfos) Remove(setB ExternalEndpointInfos) ExternalEndpointInfos {
	diffSet := ExternalEndpointInfos{}
	for _, externalEndpoint := range setA {
		if !containsExternalPort(setB, externalEndpoint.Port) {
			diffSet = append(diffSet, ExternalEndpointInfo{externalEndpoint.RouterGroupGUID, externalEndpoint.Port})
		}
	}
	return diffSet
}

func (lhs RoutingKeys) Remove(rhs RoutingKeys) RoutingKeys {
	result := RoutingKeys{}
	for _, lhsKey := range lhs {
		if !rhs.containsRoutingKey(lhsKey) {
			result = append(result, lhsKey)
		}
	}
	return result
}

func (lhs RoutingKeys) containsRoutingKey(routingKey RoutingKey) bool {
	for _, lhsKey := range lhs {
		if lhsKey == routingKey {
			return true
		}
	}
	return false
}
