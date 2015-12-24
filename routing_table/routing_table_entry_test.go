package routing_table_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingTableEntry", func() {
	var (
		source routing_table.ExternalEndpointInfos
	)
	BeforeEach(func() {
		source = routing_table.ExternalEndpointInfos{
			routing_table.NewExternalEndpointInfo("routing-group-1", 6000),
			routing_table.NewExternalEndpointInfo("routing-group-1", 6100),
		}
	})

	Context("Remove", func() {
		Context("when removing all the current elements", func() {
			It("returns an empty set", func() {
				deletingSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				Expect(resultSet).Should(Equal(routing_table.ExternalEndpointInfos{}))
			})
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				expectedSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing none of the current elements", func() {
			It("returns the same set", func() {
				deletingSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6200},
				}
				resultSet := source.Remove(deletingSet)
				expectedSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing an empty set", func() {
			It("returns the same set", func() {
				deletingSet := routing_table.ExternalEndpointInfos{}
				resultSet := source.Remove(deletingSet)
				expectedSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing from an empty set", func() {
			It("returns the same set", func() {
				source = routing_table.ExternalEndpointInfos{}
				deletingSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				Expect(resultSet).Should(Equal(routing_table.ExternalEndpointInfos{}))
			})
		})
	})

	Context("RemoveExternalEndpoints", func() {
		var (
			sourceEntry     routing_table.RoutableEndpoints
			modificationTag *models.ModificationTag
			endpoints       map[routing_table.EndpointKey]routing_table.Endpoint
		)
		BeforeEach(func() {
			endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			sourceEntry = routing_table.NewRoutableEndpoints(source, endpoints, "log-guid-1", modificationTag)
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultEntry := sourceEntry.RemoveExternalEndpoints(deletingSet)
				expectedSet := routing_table.ExternalEndpointInfos{
					routing_table.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
				}
				expectedEntry := routing_table.NewRoutableEndpoints(expectedSet, endpoints, "log-guid-1", modificationTag)
				Expect(resultEntry).Should(Equal(expectedEntry))
			})
		})
	})

	Context("RoutingKeys Remove", func() {
		var (
			sourceRoutingKeys routing_table.RoutingKeys
		)

		BeforeEach(func() {
			sourceRoutingKeys = routing_table.RoutingKeys{
				routing_table.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
				routing_table.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
			}
		})

		Context("when removing all the current elements", func() {
			It("returns an empty set", func() {
				deletingRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					routing_table.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				Expect(resultSet).Should(Equal(routing_table.RoutingKeys{}))
			})
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing none of the current elements", func() {
			It("returns the same set", func() {
				deletingRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-3", ContainerPort: 5002},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					routing_table.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing an empty set", func() {
			It("returns the same set", func() {
				deletingRoutingKeys := routing_table.RoutingKeys{}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					routing_table.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing from an empty set", func() {
			It("returns the same set", func() {
				sourceRoutingKeys = routing_table.RoutingKeys{}
				deletingRoutingKeys := routing_table.RoutingKeys{
					routing_table.RoutingKey{ProcessGUID: "process-guid-3", ContainerPort: 5002},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				Expect(resultSet).Should(Equal(routing_table.RoutingKeys{}))
			})
		})
	})

})
