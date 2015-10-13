package routing_table

import (
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

	token, err := emitter.tokenFetcher.FetchToken()
	if err != nil {
		emitter.logger.Error("unable-to-get-token", err)
		return err
	}

	registrationMappingRequests, unregistrationMappingRequests := CreateMappingRequests(emitter.logger, routingEvents)

	emitter.routingApiClient.SetToken(token.AccessToken)

	emitted := true
	if len(registrationMappingRequests) > 0 {
		if err = emitter.routingApiClient.UpsertTcpRouteMappings(registrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-upsert", err)
		}
	}

	if len(unregistrationMappingRequests) > 0 {
		if err = emitter.routingApiClient.DeleteTcpRouteMappings(unregistrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-delete", err)
		}
	}

	if emitted {
		emitter.logger.Debug("successfully-emitted-events")
	}
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
