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
