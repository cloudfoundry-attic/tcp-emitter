package routing_table_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingTableHelpers", func() {
	Describe("EndpointsFromActual", func() {
		It("builds a map of container port to endpoint", func() {
			tag := receptor.ModificationTag{Epoch: "abc", Index: 0}
			endpoints, err := routing_table.EndpointsFromActual(receptor.ActualLRPResponse{
				ProcessGuid:  "process-guid",
				InstanceGuid: "instance-guid",
				Index:        0,
				Domain:       "domain",
				Address:      "1.1.1.1",
				Ports: []receptor.PortMapping{
					{HostPort: 11, ContainerPort: 44},
					{HostPort: 66, ContainerPort: 99},
				},
				Evacuating:      true,
				ModificationTag: tag,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(endpoints).To(ConsistOf([]routing_table.Endpoint{
				routing_table.NewEndpoint("instance-guid", true, "1.1.1.1", 11, 44, tag),
				routing_table.NewEndpoint("instance-guid", true, "1.1.1.1", 66, 99, tag),
			}))
		})
	})

	Describe("RoutingKeysFromActual", func() {
		It("creates a list of keys for an actual LRP", func() {
			keys := routing_table.RoutingKeysFromActual(receptor.ActualLRPResponse{
				ProcessGuid:  "process-guid",
				InstanceGuid: "instance-guid",
				Index:        0,
				Domain:       "domain",
				Address:      "1.1.1.1",
				Ports: []receptor.PortMapping{
					{HostPort: 11, ContainerPort: 44},
					{HostPort: 66, ContainerPort: 99},
				},
			})

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routing_table.NewRoutingKey("process-guid", 44)))
			Expect(keys).To(ContainElement(routing_table.NewRoutingKey("process-guid", 99)))
		})

		Context("when the actual lrp has no port mappings", func() {
			It("returns no keys", func() {
				keys := routing_table.RoutingKeysFromActual(receptor.ActualLRPResponse{
					ProcessGuid:  "process-guid",
					InstanceGuid: "instance-guid",
					Index:        0,
					Domain:       "domain",
					Address:      "1.1.1.1",
				})

				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("RoutingKeysFromDesired", func() {
		It("creates a list of keys for an actual LRP", func() {
			routes := tcp_routes.TCPRoutes{
				{ExternalPort: 61000, ContainerPort: 8080},
				{ExternalPort: 61001, ContainerPort: 9090},
			}

			desired := receptor.DesiredLRPResponse{
				Domain:      "tests",
				ProcessGuid: "process-guid",
				Ports:       []uint16{8080, 9090},
				Routes:      routes.RoutingInfo(),
				LogGuid:     "abc-guid",
			}

			keys := routing_table.RoutingKeysFromDesired(desired)

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routing_table.NewRoutingKey("process-guid", 8080)))
			Expect(keys).To(ContainElement(routing_table.NewRoutingKey("process-guid", 9090)))
		})

		Context("when the desired LRP does not define any container ports", func() {
			It("returns no keys", func() {
				desired := receptor.DesiredLRPResponse{
					Domain:      "tests",
					ProcessGuid: "process-guid",
					Routes:      tcp_routes.TCPRoutes{{ExternalPort: 61000, ContainerPort: 8080}}.RoutingInfo(),
					LogGuid:     "abc-guid",
				}

				keys := routing_table.RoutingKeysFromDesired(desired)
				Expect(keys).To(HaveLen(0))
			})
		})
	})
})
