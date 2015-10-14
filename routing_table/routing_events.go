package routing_table

type RoutingEventType string

const (
	RouteRegistrationEvent   RoutingEventType = "RouteRegistrationEvent"
	RouteUnregistrationEvent RoutingEventType = "RouteUnregistrationEvent"
)

type RoutingEvent struct {
	EventType RoutingEventType
	Key       RoutingKey
	Entry     RoutableEndpoints
}

type RoutingEvents []RoutingEvent

func (r RoutingEvent) Valid() bool {
	if len(r.Entry.Endpoints) == 0 {
		return false
	}
	if len(r.Entry.ExternalEndpoints) == 0 {
		return false
	}
	for _, externalEndpoint := range r.Entry.ExternalEndpoints {
		if externalEndpoint.Port == 0 {
			return false
		}
	}
	return true
}
