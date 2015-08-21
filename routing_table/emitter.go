package routing_table

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	Emit(routingEvents RoutingEvents) error
}

type tcpEmitter struct {
	logger          lager.Logger
	tcpRouterAPIURL string
}

func NewEmitter(logger lager.Logger, tcpRouterAPIURL string) Emitter {
	return &tcpEmitter{
		logger:          logger,
		tcpRouterAPIURL: tcpRouterAPIURL,
	}
}

func (emitter *tcpEmitter) Emit(routingEvents RoutingEvents) error {

	emitter.logRoutingEvents(routingEvents)

	mappingRequests := BuildMappingRequests(routingEvents)
	if len(mappingRequests) == 0 {
		err := errors.New(fmt.Sprintf("Unable to build mapping request"))
		emitter.logger.Error("no-mapping-request-to-emit", err)
		return err
	}
	payload, err := json.Marshal(mappingRequests)
	if err != nil {
		emitter.logger.Error("invalid-mapping-request-marshaling", err)
		return err
	}
	resp, err := http.Post(
		fmt.Sprintf("%s/v0/external_ports", emitter.tcpRouterAPIURL),
		"application/json", bytes.NewBuffer(payload))

	if err != nil {
		emitter.logger.Error("unable-to-post", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf("Received non OK status code %d", resp.StatusCode))
		emitter.logger.Error("unable-to-post", err)
		return err
	}

	return nil
}

func (emitter *tcpEmitter) logRoutingEvents(routingEvents RoutingEvents) {
	for _, event := range routingEvents {
		endpoints := make([]Endpoint, 0)
		for _, endpoint := range event.Entry.Endpoints {
			endpoints = append(endpoints, endpoint)
		}

		ports := make([]uint16, 0)
		for _, extEndpoint := range event.Entry.ExternalEndpoints {
			ports = append(ports, extEndpoint.Port)
		}
		emitter.logger.Info("mapped-routes", lager.Data{
			"external_ports": ports,
			"backends":       endpoints})
	}
}
