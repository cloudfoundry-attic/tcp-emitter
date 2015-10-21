package routing_table

import (
	"errors"

	"github.com/cloudfoundry-incubator/bbs/models"
)

func EndpointsFromActual(actualGrp *models.ActualLRPGroup) (map[uint32]Endpoint, error) {
	endpoints := map[uint32]Endpoint{}
	actual, evacuating := actualGrp.Resolve()

	if len(actual.Ports) == 0 {
		return endpoints, errors.New("missing ports")
	}

	for _, portMapping := range actual.Ports {
		endpoint := NewEndpoint(
			actual.InstanceGuid, evacuating,
			actual.Address,
			portMapping.HostPort,
			portMapping.ContainerPort,
			&actual.ModificationTag,
		)
		endpoints[portMapping.ContainerPort] = endpoint
	}

	return endpoints, nil
}

func RoutingKeysFromActual(actualGrp *models.ActualLRPGroup) RoutingKeys {
	keys := RoutingKeys{}
	actual, _ := actualGrp.Resolve()
	for _, portMapping := range actual.Ports {
		keys = append(keys, NewRoutingKey(actual.ProcessGuid, portMapping.ContainerPort))
	}

	return keys
}

func RoutingKeysFromDesired(desired *models.DesiredLRP) RoutingKeys {
	keys := RoutingKeys{}
	for _, containerPort := range desired.Ports {
		keys = append(keys, NewRoutingKey(desired.ProcessGuid, containerPort))
	}

	return keys
}
