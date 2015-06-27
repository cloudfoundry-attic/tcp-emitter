package routing_table

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	Emit(routingEvents RoutingEvents) error
}

type tcpEmitter struct {
	logger lager.Logger
}

func (emitter *tcpEmitter) Emit(routingEvents RoutingEvents) error {
	emitter.logRoutingEvents(routingEvents)
	return nil
}

func NewEmitter(logger lager.Logger) Emitter {
	return &tcpEmitter{
		logger: logger,
	}
}

func (emitter *tcpEmitter) logRoutingEvents(routingEvents RoutingEvents) {
	for _, event := range routingEvents {
		endpoints := make([]Endpoint, 0)
		for _, endpoint := range event.Entry.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
		emitter.logger.Info("mapped-routes", lager.Data{
			"external_port": event.Entry.ExternalEndpoint.Port,
			"backends":      endpoints})
	}
}
