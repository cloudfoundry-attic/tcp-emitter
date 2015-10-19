package routing_table_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/cloudfoundry-incubator/routing-api/fake_routing_api"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/uaa-token-fetcher"
	testTokenFetcher "github.com/cloudfoundry-incubator/uaa-token-fetcher/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Emitter", func() {

	var (
		emitter                 routing_table.Emitter
		routingApiClient        *fake_routing_api.FakeClient
		tokenFetcher            *testTokenFetcher.FakeTokenFetcher
		routingEvents           routing_table.RoutingEvents
		expectedMappingRequests []db.TcpRouteMapping
		routingKey1             routing_table.RoutingKey
		routableEndpoints1      routing_table.RoutableEndpoints
	)

	BeforeEach(func() {
		logGuid := "log-guid-1"
		modificationTag := models.ModificationTag{Epoch: "abc", Index: 0}

		endpoints1 := map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62003, 5222, &modificationTag),
		}

		routingKey1 = routing_table.NewRoutingKey("process-guid-1", 5222)

		extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo("123", 61000)

		expectedMappingRequests = []db.TcpRouteMapping{
			db.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003),
		}

		routableEndpoints1 = routing_table.NewRoutableEndpoints(
			routing_table.ExternalEndpointInfos{extenralEndpointInfo1}, endpoints1, logGuid, &modificationTag)

		routingEvents = routing_table.RoutingEvents{
			routing_table.RoutingEvent{
				EventType: routing_table.RouteRegistrationEvent,
				Key:       routingKey1,
				Entry:     routableEndpoints1,
			},
		}

		routingApiClient = new(fake_routing_api.FakeClient)
		tokenFetcher = &testTokenFetcher.FakeTokenFetcher{}
		tokenFetcher.FetchTokenReturns(&token_fetcher.Token{"some-token", 1234}, nil)
		emitter = routing_table.NewEmitter(logger, routingApiClient, tokenFetcher)
	})

	Context("when valid routing events are provided", func() {

		Context("when router API returns no errors", func() {

			Context("and there are registration events", func() {
				It("emits valid upsert tcp routes", func() {
					err := emitter.Emit(routingEvents)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(routingApiClient.UpsertTcpRouteMappingsCallCount()).To(Equal(1))
					Expect(tokenFetcher.FetchTokenCallCount()).To(Equal(1))
					Expect(routingApiClient.SetTokenCallCount()).To(Equal(1))
					Expect(routingApiClient.SetTokenArgsForCall(0)).To(Equal("some-token"))
					mappingRequests := routingApiClient.UpsertTcpRouteMappingsArgsForCall(0)
					Expect(mappingRequests).To(ConsistOf(expectedMappingRequests))
				})
			})
			Context("and there are unregistration events", func() {
				BeforeEach(func() {
					routingEvents = routing_table.RoutingEvents{
						routing_table.RoutingEvent{
							EventType: routing_table.RouteUnregistrationEvent,
							Key:       routingKey1,
							Entry:     routableEndpoints1,
						},
					}
				})
				It("emits valid delete tcp routes", func() {
					err := emitter.Emit(routingEvents)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(routingApiClient.DeleteTcpRouteMappingsCallCount()).To(Equal(1))
					Expect(tokenFetcher.FetchTokenCallCount()).To(Equal(1))
					Expect(routingApiClient.SetTokenCallCount()).To(Equal(1))
					Expect(routingApiClient.SetTokenArgsForCall(0)).To(Equal("some-token"))
					mappingRequests := routingApiClient.DeleteTcpRouteMappingsArgsForCall(0)
					Expect(mappingRequests).To(ConsistOf(expectedMappingRequests))
				})
			})
		})

		Context("when routing API Upsert returns error", func() {
			BeforeEach(func() {
				routingApiClient.UpsertTcpRouteMappingsReturns(errors.New("kabooom"))
			})

			It("returns error", func() {
				err := emitter.Emit(routingEvents)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("test.unable-to-upsert"))
			})
		})

		Context("when routing API Delete returns error", func() {

			BeforeEach(func() {
				routingApiClient.DeleteTcpRouteMappingsReturns(errors.New("kabooom"))
				routingEvents = routing_table.RoutingEvents{
					routing_table.RoutingEvent{
						EventType: routing_table.RouteUnregistrationEvent,
						Key:       routingKey1,
						Entry:     routableEndpoints1,
					},
				}
			})

			It("returns error", func() {
				err := emitter.Emit(routingEvents)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("test.unable-to-delete"))
			})
		})

		Context("when token fetcher returns error", func() {
			BeforeEach(func() {
				tokenFetcher.FetchTokenReturns(nil, errors.New("kabooom"))
			})

			It("returns error", func() {
				err := emitter.Emit(routingEvents)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(Equal("kabooom"))
				Expect(logger).To(gbytes.Say("test.unable-to-get-token"))
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

			extenralEndpointInfo1 := routing_table.NewExternalEndpointInfo("123", 0)

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
			Expect(err).ShouldNot(HaveOccurred())
			Expect(logger).To(gbytes.Say("test.invalid-routing-event"))
		})
	})

})
