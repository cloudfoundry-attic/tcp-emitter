package routing_table

import (
	"github.com/cloudfoundry-incubator/routing-api"
	"github.com/cloudfoundry-incubator/routing-api/db"
	token_fetcher "github.com/cloudfoundry-incubator/uaa-token-fetcher"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	Emit(routingEvents RoutingEvents) error
}

type tcpEmitter struct {
	logger           lager.Logger
	routingAPIClient routing_api.Client
	tokenFetcher     token_fetcher.TokenFetcher
}

func NewEmitter(logger lager.Logger, routingAPIClient routing_api.Client, tokenFetcher token_fetcher.TokenFetcher) Emitter {
	return &tcpEmitter{
		logger:           logger,
		routingAPIClient: routingAPIClient,
		tokenFetcher:     tokenFetcher,
	}
}

func (emitter *tcpEmitter) Emit(routingEvents RoutingEvents) error {
	emitter.logRoutingEvents(routingEvents)
	defer emitter.logger.Debug("complete-emit")

	registrationMappingRequests, unregistrationMappingRequests := CreateMappingRequests(emitter.logger, routingEvents)
	useCachedToken := true
	for count := 0; count < 2; count++ {
		token, err := emitter.tokenFetcher.FetchToken(useCachedToken)
		if err != nil {
			emitter.logger.Error("unable-to-get-token", err)
			return err
		}
		emitter.routingAPIClient.SetToken(token.AccessToken)
		err = emitter.emit(registrationMappingRequests, unregistrationMappingRequests)
		if err != nil && err.Error() == "unauthorized" {
			useCachedToken = false
			emitter.logger.Info("retrying-emit")
		} else {
			break
		}
	}

	return nil
}

func (emitter *tcpEmitter) emit(registrationMappingRequests, unregistrationMappingRequests []db.TcpRouteMapping) error {
	emitted := true
	if len(registrationMappingRequests) > 0 {
		if err := emitter.routingAPIClient.UpsertTcpRouteMappings(registrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-upsert", err)
			return err
		}
		emitter.logger.Debug("successfully-emitted-registration-events",
			lager.Data{"number-of-registration-events": len(registrationMappingRequests)})

	}

	if len(unregistrationMappingRequests) > 0 {
		if err := emitter.routingAPIClient.DeleteTcpRouteMappings(unregistrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-delete", err)
			return err
		}
		emitter.logger.Debug("successfully-emitted-unregistration-events",
			lager.Data{"number-of-unregistration-events": len(unregistrationMappingRequests)})

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
