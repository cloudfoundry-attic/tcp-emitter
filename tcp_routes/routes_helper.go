package tcp_routes

import (
	"encoding/json"

	"github.com/cloudfoundry-incubator/receptor"
)

const TCP_ROUTER = "tcp-router"

type TCPRoutes []TCPRoute

type TCPRoute struct {
	ExternalPort  uint16 `json:"external_port"`
	ContainerPort uint16 `json:"container_port"`
}

func (c TCPRoutes) RoutingInfo() receptor.RoutingInfo {
	data, _ := json.Marshal(c)
	routingInfo := json.RawMessage(data)
	return receptor.RoutingInfo{
		TCP_ROUTER: &routingInfo,
	}
}

func TCPRoutesFromRoutingInfo(routingInfo receptor.RoutingInfo) (TCPRoutes, error) {
	if routingInfo == nil {
		return nil, nil
	}

	data, found := routingInfo[TCP_ROUTER]
	if !found {
		return nil, nil
	}

	if data == nil {
		return nil, nil
	}

	routes := TCPRoutes{}
	err := json.Unmarshal(*data, &routes)

	return routes, err
}
