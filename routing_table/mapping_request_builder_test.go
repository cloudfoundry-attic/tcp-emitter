package routing_table_test

import (
	cf_tcp_router "github.com/cloudfoundry-incubator/cf-tcp-router"
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MappingRequestBuilder", func() {

	verifyMappingRequest := func(mappingRequestMap map[uint16]cf_tcp_router.BackendHostInfos,
		port uint16, backends cf_tcp_router.BackendHostInfos) {
		Expect(mappingRequestMap).Should(HaveKey(port))
		Expect(mappingRequestMap[port]).Should(HaveLen(len(backends)))
		for _, backend := range backends {
			Expect(mappingRequestMap[port]).Should(ContainElement(backend))
		}
	}

	var (
		routingEvents           routing_table.RoutingEvents
		expectedMappingRequests cf_tcp_router.MappingRequests
		endpoints1              map[routing_table.EndpointKey]routing_table.Endpoint
		endpoints2              map[routing_table.EndpointKey]routing_table.Endpoint
		routingKey1             routing_table.RoutingKey
		routingKey2             routing_table.RoutingKey
		logGuid                 string
		modificationTag         receptor.ModificationTag
	)

	BeforeEach(func() {
		logGuid = "log-guid-1"
		modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 0}

		endpoints1 = map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62003, 5222, modificationTag),
			routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
				"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
		}
		endpoints2 = map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-3", false, "some-ip-3", 62005, 5222, modificationTag),
			routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
				"instance-guid-4", false, "some-ip-4", 62006, 5222, modificationTag),
		}

		routingKey1 = routing_table.NewRoutingKey("process-guid-1", 5222)
		routingKey2 = routing_table.NewRoutingKey("process-guid-2", 5222)

		extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(61000)
		extenralEndpointInfo2 := routing_table.NewExternalEndpointInfo(61001)

		routableEndpoints1 := routing_table.NewRoutableEndpoints(
			extenralEndpointInfo1, endpoints1, logGuid, modificationTag)
		routableEndpoints2 := routing_table.NewRoutableEndpoints(
			extenralEndpointInfo2, endpoints2, logGuid, modificationTag)

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

		expectedMappingRequests = cf_tcp_router.MappingRequests{
			cf_tcp_router.NewMappingRequest(61000, cf_tcp_router.BackendHostInfos{
				cf_tcp_router.NewBackendHostInfo("some-ip-1", 62003),
				cf_tcp_router.NewBackendHostInfo("some-ip-2", 62004),
			}),
			cf_tcp_router.NewMappingRequest(61001, cf_tcp_router.BackendHostInfos{
				cf_tcp_router.NewBackendHostInfo("some-ip-3", 62005),
				cf_tcp_router.NewBackendHostInfo("some-ip-4", 62006),
			}),
		}
	})

	Context("with valid routing events", func() {
		It("returns valid mapping requests ", func() {
			mappingRequests := routing_table.BuildMappingRequests(routingEvents)
			Expect(mappingRequests).Should(HaveLen(2))
			mappingRequestMap := make(map[uint16]cf_tcp_router.BackendHostInfos)
			for _, mappingRequest := range mappingRequests {
				mappingRequestMap[mappingRequest.ExternalPort] = mappingRequest.Backends
			}
			verifyMappingRequest(mappingRequestMap, 61000, cf_tcp_router.BackendHostInfos{
				cf_tcp_router.NewBackendHostInfo("some-ip-1", 62003),
				cf_tcp_router.NewBackendHostInfo("some-ip-2", 62004),
			})
			verifyMappingRequest(mappingRequestMap, 61001, cf_tcp_router.BackendHostInfos{
				cf_tcp_router.NewBackendHostInfo("some-ip-3", 62005),
				cf_tcp_router.NewBackendHostInfo("some-ip-4", 62006),
			})
		})
	})

	Context("with an invalid external port in routing event", func() {
		It("returns an empty mapping request", func() {

			extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(0)

			routableEndpoints1 := routing_table.NewRoutableEndpoints(
				extenralEndpointInfo1, endpoints1, logGuid, modificationTag)

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}

			mappingRequests := routing_table.BuildMappingRequests(routingEvents)
			Expect(mappingRequests).Should(HaveLen(0))
		})
	})

	Context("with empty endpoints in routing event", func() {
		It("returns an empty mapping request", func() {
			extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(61000)

			routableEndpoints1 := routing_table.NewRoutableEndpoints(
				extenralEndpointInfo1, nil, logGuid, modificationTag)

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}

			mappingRequests := routing_table.BuildMappingRequests(routingEvents)
			Expect(mappingRequests).Should(HaveLen(0))
		})
	})

})
