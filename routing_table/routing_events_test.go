package routing_table_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingEvent", func() {
	var (
		routingEvent      routing_table.RoutingEvent
		routingKey        routing_table.RoutingKey
		modificationTag   *models.ModificationTag
		endpoints         map[routing_table.EndpointKey]routing_table.Endpoint
		routableEndpoints routing_table.RoutableEndpoints
		logGuid           string
	)
	BeforeEach(func() {
		routingKey = routing_table.NewRoutingKey("process-guid-1", 5222)
		logGuid = "log-guid-1"
		modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
		endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
			routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
			routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
				"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
		}
	})

	Context("Valid", func() {
		Context("valid routing event is passed", func() {
			BeforeEach(func() {
				externalEndpoints := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("some-guid", 61000),
				}
				routableEndpoints = routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = routing_table.RoutingEvent{routing_table.RouteRegistrationEvent, routingKey, routableEndpoints}
			})
			It("returns true", func() {
				Expect(routingEvent.Valid()).To(BeTrue())
			})
		})

		Context("routing event with empty endpoints is passed", func() {
			BeforeEach(func() {
				externalEndpoints := routing_table.ExternalEndpointInfos{}
				routableEndpoints = routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = routing_table.RoutingEvent{routing_table.RouteRegistrationEvent, routingKey, routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})

		Context("routing event with one external port is zero", func() {
			BeforeEach(func() {
				externalEndpoints := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("router-guid", 0),
				}
				routableEndpoints = routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = routing_table.RoutingEvent{routing_table.RouteRegistrationEvent, routingKey, routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})

		Context("routing event with no endpoints", func() {
			BeforeEach(func() {
				externalEndpoints := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("r-g", 61000),
				}
				routableEndpoints = routing_table.NewRoutableEndpoints(externalEndpoints, map[routing_table.EndpointKey]routing_table.Endpoint{},
					logGuid, modificationTag)
				routingEvent = routing_table.RoutingEvent{routing_table.RouteRegistrationEvent, routingKey, routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})
	})
})
