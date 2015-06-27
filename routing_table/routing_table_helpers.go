package routing_table

import (
	"errors"

	"github.com/cloudfoundry-incubator/receptor"
)

func EndpointsFromActual(actual receptor.ActualLRPResponse) (map[uint16]Endpoint, error) {
	endpoints := map[uint16]Endpoint{}

	if len(actual.Ports) == 0 {
		return endpoints, errors.New("missing ports")
	}

	for _, portMapping := range actual.Ports {
		endpoint := NewEndpoint(
			actual.InstanceGuid, actual.Evacuating,
			actual.Address,
			portMapping.HostPort,
			portMapping.ContainerPort,
			actual.ModificationTag,
		)
		endpoints[portMapping.ContainerPort] = endpoint
	}

	return endpoints, nil
}

func RoutingKeysFromActual(actual receptor.ActualLRPResponse) []RoutingKey {
	keys := []RoutingKey{}
	for _, portMapping := range actual.Ports {
		keys = append(keys, NewRoutingKey(actual.ProcessGuid, portMapping.ContainerPort))
	}

	return keys
}

func RoutingKeysFromDesired(desired receptor.DesiredLRPResponse) []RoutingKey {
	keys := []RoutingKey{}
	for _, containerPort := range desired.Ports {
		keys = append(keys, NewRoutingKey(desired.ProcessGuid, containerPort))
	}

	return keys
}
