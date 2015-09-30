package routing_table

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry-incubator/routing-api"
	token_fetcher "github.com/cloudfoundry-incubator/uaa-token-fetcher"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	Emit(routingEvents RoutingEvents) error
}

type tcpEmitter struct {
	logger           lager.Logger
	routingApiClient routing_api.Client
	tokenFetcher     token_fetcher.TokenFetcher
}

func NewEmitter(logger lager.Logger, routingApiClient routing_api.Client, tokenFetcher token_fetcher.TokenFetcher) Emitter {
	return &tcpEmitter{
		logger:           logger,
		routingApiClient: routingApiClient,
		tokenFetcher:     tokenFetcher,
	}
}

func (emitter *tcpEmitter) Emit(routingEvents RoutingEvents) error {

	emitter.logRoutingEvents(routingEvents)
	defer emitter.logger.Debug("complete-emit")

	mappingRequests := BuildMappingRequests(routingEvents)
	if len(mappingRequests) == 0 {
		err := errors.New(fmt.Sprintf("Unable to build mapping request"))
		emitter.logger.Error("no-mapping-request-to-emit", err)
		return err
	}

	token, err := emitter.tokenFetcher.FetchToken()
	if err != nil {
		emitter.logger.Error("unable-to-get-token", err)
		return err
	}

	emitter.routingApiClient.SetToken(token.AccessToken)

	err = emitter.routingApiClient.UpsertTcpRouteMappings(mappingRequests)
	if err != nil {
		emitter.logger.Error("unable-to-upsert", err)
		return err
	}
	emitter.logger.Debug("successfully-upserted-event")

	return nil
}

func (emitter *tcpEmitter) logRoutingEvents(routingEvents RoutingEvents) {
	for _, event := range routingEvents {
		endpoints := make([]Endpoint, 0)
		for _, endpoint := range event.Entry.Endpoints {
			endpoints = append(endpoints, endpoint)
		}

		ports := make([]uint32, 0)
		for _, extEndpoint := range event.Entry.ExternalEndpoints {
			ports = append(ports, extEndpoint.Port)
		}
		emitter.logger.Info("mapped-routes", lager.Data{
			"external_ports": ports,
			"backends":       endpoints})
	}
}
