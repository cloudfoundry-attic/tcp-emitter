package routing_table_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MappingRequestBuilder", func() {

	var (
		routingEvents                  routing_table.RoutingEvents
		expectedRegistrationRequests   []db.TcpRouteMapping
		expectedUnregistrationRequests []db.TcpRouteMapping
		endpoints1                     map[routing_table.EndpointKey]routing_table.Endpoint
		endpoints2                     map[routing_table.EndpointKey]routing_table.Endpoint
		routingKey1                    routing_table.RoutingKey
		routingKey2                    routing_table.RoutingKey
		routableEndpoints1             routing_table.RoutableEndpoints
		routableEndpoints2             routing_table.RoutableEndpoints
		logGuid                        string
		modificationTag                models.ModificationTag
	)

	BeforeEach(func() {
		logGuid = "log-guid-1"
		modificationTag = models.ModificationTag{Epoch: "abc", Index: 0}

		endpoints1 = map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62003, 5222, &modificationTag),
			routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
				"instance-guid-2", false, "some-ip-2", 62004, 5222, &modificationTag),
		}
		endpoints2 = map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-3", false, "some-ip-3", 62005, 5222, &modificationTag),
			routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
				"instance-guid-4", false, "some-ip-4", 62006, 5222, &modificationTag),
		}

		routingKey1 = routing_table.NewRoutingKey("process-guid-1", 5222)
		routingKey2 = routing_table.NewRoutingKey("process-guid-2", 5222)

		extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(61000)
		extenralEndpointInfo2 := routing_table.NewExternalEndpointInfo(61001)
		extenralEndpointInfo3 := routing_table.NewExternalEndpointInfo(61002)
		endpointInfo1 := routing_table.ExternalEndpointInfos{extenralEndpointInfo1}
		endpointInfo2 := routing_table.ExternalEndpointInfos{
			extenralEndpointInfo2,
			extenralEndpointInfo3,
		}

		routableEndpoints1 = routing_table.NewRoutableEndpoints(endpointInfo1, endpoints1, logGuid, &modificationTag)
		routableEndpoints2 = routing_table.NewRoutableEndpoints(endpointInfo2, endpoints2, logGuid, &modificationTag)
	})

	Context("with valid routing events", func() {
		BeforeEach(func() {
			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				routing_table.RoutingEvent{
					EventType: routing_table.RouteUnregistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedRegistrationRequests = []db.TcpRouteMapping{
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-1", 62003),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-2", 62004),
			}

			expectedUnregistrationRequests = []db.TcpRouteMapping{
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-4", 62006),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-4", 62006),
			}
		})

		It("returns valid registration and unregistration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
			Expect(registrationRequests).Should(HaveLen(len(expectedRegistrationRequests)))
			Expect(registrationRequests).Should(ConsistOf(expectedRegistrationRequests))
			Expect(unregistrationRequests).Should(HaveLen(len(expectedUnregistrationRequests)))
			Expect(unregistrationRequests).Should(ConsistOf(expectedUnregistrationRequests))
		})
	})

	Context("with no unregistration events", func() {
		BeforeEach(func() {
			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedRegistrationRequests = []db.TcpRouteMapping{
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-1", 62003),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-2", 62004),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-4", 62006),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-4", 62006),
			}

			expectedUnregistrationRequests = []db.TcpRouteMapping{}
		})

		It("returns only registration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
			Expect(registrationRequests).Should(HaveLen(len(expectedRegistrationRequests)))
			Expect(registrationRequests).Should(ConsistOf(expectedRegistrationRequests))
			Expect(unregistrationRequests).Should(HaveLen(0))
		})
	})

	Context("with no registration events", func() {
		BeforeEach(func() {
			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteUnregistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				routing_table.RoutingEvent{
					EventType: routing_table.RouteUnregistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedUnregistrationRequests = []db.TcpRouteMapping{
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-1", 62003),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61000, "some-ip-2", 62004),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61001, "some-ip-4", 62006),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-3", 62005),
				db.NewTcpRouteMapping(routing_table.DefaultRouterGroupGuid, 61002, "some-ip-4", 62006),
			}

			expectedRegistrationRequests = []db.TcpRouteMapping{}
		})

		It("returns only unregistration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
			Expect(unregistrationRequests).Should(HaveLen(len(expectedUnregistrationRequests)))
			Expect(unregistrationRequests).Should(ConsistOf(expectedUnregistrationRequests))
			Expect(registrationRequests).Should(HaveLen(0))
		})
	})

	Context("with an invalid external port in route registration event", func() {

		It("returns an empty registration request", func() {
			extenralEndpointInfo1 := routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo(0),
			}

			routableEndpoints1 := routing_table.NewRoutableEndpoints(
				extenralEndpointInfo1, endpoints1, logGuid, &modificationTag)

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}
			registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
			Expect(unregistrationRequests).Should(HaveLen(0))
			Expect(registrationRequests).Should(HaveLen(0))
		})

		Context("and multiple external ports", func() {
			It("disregards the entire routing event", func() {
				extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(0)
				extenralEndpointInfo2 := routing_table.NewExternalEndpointInfo(61000)
				externalInfo := []routing_table.ExternalEndpointInfo{
					extenralEndpointInfo1,
					extenralEndpointInfo2,
				}

				routableEndpoints1 := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints1, logGuid, &modificationTag)

				routingEvents = routing_table.RoutingEvents{
					routing_table.RoutingEvent{
						EventType: routing_table.RouteRegistrationEvent,
						Key:       routingKey1,
						Entry:     routableEndpoints1,
					},
				}

				registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
				Expect(unregistrationRequests).Should(HaveLen(0))
				Expect(registrationRequests).Should(HaveLen(0))
			})
		})
	})

	Context("with empty endpoints in routing event", func() {
		It("returns an empty mapping request", func() {
			extenralEndpointInfo1 := routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo(0),
			}

			routableEndpoints1 := routing_table.NewRoutableEndpoints(
				extenralEndpointInfo1, nil, logGuid, &modificationTag)

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}

			registrationRequests, unregistrationRequests := routing_table.CreateMappingRequests(logger, routingEvents)
			Expect(unregistrationRequests).Should(HaveLen(0))
			Expect(registrationRequests).Should(HaveLen(0))
		})
	})
})
