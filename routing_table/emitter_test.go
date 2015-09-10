package routing_table_test

import (
	"encoding/json"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/cf-tcp-router"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Emitter", func() {

	var (
		emitter       routing_table.Emitter
		routingEvents routing_table.RoutingEvents
		fakeRouterAPI *ghttp.Server
	)

	BeforeEach(func() {
		logGuid := "log-guid-1"
		modificationTag := models.ModificationTag{Epoch: "abc", Index: 0}

		endpoints1 := map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62003, 5222, &modificationTag),
		}

		routingKey1 := routing_table.NewRoutingKey("process-guid-1", 5222)

		extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(61000)

		routableEndpoints1 := routing_table.NewRoutableEndpoints(
			routing_table.ExternalEndpointInfos{extenralEndpointInfo1}, endpoints1, logGuid, &modificationTag)

		routingEvents = routing_table.RoutingEvents{
			routing_table.RoutingEvent{
				EventType: routing_table.RouteRegistrationEvent,
				Key:       routingKey1,
				Entry:     routableEndpoints1,
			},
		}

		fakeRouterAPI = ghttp.NewServer()
		emitter = routing_table.NewEmitter(logger, fakeRouterAPI.URL())
	})

	AfterEach(func() {
		defer fakeRouterAPI.Close()
	})

	Context("when valid routing events are provided", func() {

		Context("when router API returns no errors", func() {
			BeforeEach(func() {
				expectedMappingRequests := cf_tcp_router.MappingRequests{
					cf_tcp_router.NewMappingRequest(61000, cf_tcp_router.BackendHostInfos{
						cf_tcp_router.NewBackendHostInfo("some-ip-1", 62003),
					}),
				}

				expectedRequestPayload, err := json.Marshal(expectedMappingRequests)
				Expect(err).ShouldNot(HaveOccurred())
				fakeRouterAPI.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v0/external_ports"),
					ghttp.VerifyJSON(string(expectedRequestPayload)),
					ghttp.RespondWith(200, ""),
				))
			})

			It("emits valid mapping request", func() {
				err := emitter.Emit(routingEvents)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when router API returns non OK status code", func() {
			BeforeEach(func() {
				fakeRouterAPI.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v0/external_ports"),
					ghttp.RespondWith(500, ""),
				))
			})

			It("emits valid mapping request", func() {
				err := emitter.Emit(routingEvents)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(Equal("Received non OK status code 500"))
			})
		})

	})

	Context("when invalid routing events are provided", func() {
		BeforeEach(func() {
			logGuid := "log-guid-1"
			modificationTag := models.ModificationTag{Epoch: "abc", Index: 0}

			endpoints1 := map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62003, 5222, &modificationTag),
			}

			routingKey1 := routing_table.NewRoutingKey("process-guid-1", 5222)

			extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo(0)

			routableEndpoints1 := routing_table.NewRoutableEndpoints(
				routing_table.ExternalEndpointInfos{extenralEndpointInfo1}, endpoints1, logGuid, &modificationTag)

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}
		})

		It("returns \"Unable to build mapping request\" error", func() {
			err := emitter.Emit(routingEvents)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(Equal("Unable to build mapping request"))
		})
	})

})
