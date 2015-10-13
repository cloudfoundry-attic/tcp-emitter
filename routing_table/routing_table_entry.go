package routing_table

import "github.com/cloudfoundry-incubator/bbs/models"

const (
	DefaultRouterGroupGuid = "bad25cff-9332-48a6-8603-b619858e7992"
)

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
	Port            uint32
	ContainerPort   uint32
	Evacuating      bool
	ModificationTag *models.ModificationTag
}

func (e Endpoint) key() EndpointKey {
	return EndpointKey{InstanceGuid: e.InstanceGuid, Evacuating: e.Evacuating}
}

func NewEndpoint(
	instanceGuid string, evacuating bool,
	host string, port, containerPort uint32,
	modificationTag *models.ModificationTag) Endpoint {
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
	RouterGroupGuid string
	Port            uint32
}

type ExternalEndpointInfos []ExternalEndpointInfo

func NewExternalEndpointInfo(port uint32) ExternalEndpointInfo {
	return ExternalEndpointInfo{
		RouterGroupGuid: DefaultRouterGroupGuid,
		Port:            port,
	}
}

type RoutableEndpoints struct {
	ExternalEndpoints ExternalEndpointInfos
	Endpoints         map[EndpointKey]Endpoint
	LogGuid           string
	ModificationTag   *models.ModificationTag
}

func (entry RoutableEndpoints) copy() RoutableEndpoints {
	clone := RoutableEndpoints{
		ExternalEndpoints: entry.ExternalEndpoints,
		Endpoints:         map[EndpointKey]Endpoint{},
		LogGuid:           entry.LogGuid,
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
	logGuid string,
	modificationTag *models.ModificationTag) RoutableEndpoints {
	return RoutableEndpoints{
		ExternalEndpoints: externalEndPoint,
		Endpoints:         endpoints,
		LogGuid:           logGuid,
		ModificationTag:   modificationTag,
	}
}

type RoutingKey struct {
	ProcessGuid   string
	ContainerPort uint32
}

func NewRoutingKey(processGuid string, containerPort uint32) RoutingKey {
	return RoutingKey{
		ProcessGuid:   processGuid,
		ContainerPort: containerPort,
	}
}

// this function returns the entryA with the external externalEndpoints substracted from its internal collection
// Ex; Given, entryA { externalEndpoints={p1,p2,p4} } and externalEndpoints = {p2,p3} ==> entryA { externalEndpoints={p1,p4} }
func (entryA RoutableEndpoints) SubstractExternalEndpoints(externalEndpoints ExternalEndpointInfos) (RoutableEndpoints, bool) {

	subtractedExternalEndpoints := entryA.ExternalEndpoints.Subtract(externalEndpoints)

	if len(subtractedExternalEndpoints) > 0 {
		resultEntry := entryA.copy()
		resultEntry.ExternalEndpoints = subtractedExternalEndpoints
		return resultEntry, true
	}

	return entryA, false
}

// this function return a-b set. Ex: a = {p1,p2, p4} b={p2,p3} ===> a-b = {p1, p4}
func (setA ExternalEndpointInfos) Subtract(setB ExternalEndpointInfos) ExternalEndpointInfos {
	var diffSet ExternalEndpointInfos
	for _, externalEndpoint := range setA {
		if !containsExternalPort(setB, externalEndpoint.Port) {
			diffSet = append(diffSet, NewExternalEndpointInfo(externalEndpoint.Port))
		}
	}
	return diffSet
}
