package routing_table_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingTable", func() {

	var (
		routingTable    routing_table.RoutingTable
		modificationTag receptor.ModificationTag
		tcpRoutes       tcp_routes.TCPRoutes
	)

	getDesiredLRP := func(processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes, modificationTag receptor.ModificationTag) receptor.DesiredLRPResponse {
		var desiredLRP receptor.DesiredLRPResponse
		portMap := map[uint16]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint16{}
		for k, _ := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint16, evacuating bool,
		modificationTag receptor.ModificationTag) receptor.ActualLRPResponse {
		actualLRP := receptor.ActualLRPResponse{
			ProcessGuid:     processGuid,
			InstanceGuid:    instanceGuid,
			Address:         hostAddress,
			Evacuating:      evacuating,
			ModificationTag: modificationTag,
			Ports:           []receptor.PortMapping{{ContainerPort: containerPort, HostPort: hostPort}},
		}
		return actualLRP
	}

	BeforeEach(func() {
		tcpRoutes = tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  61000,
				ContainerPort: 5222,
			},
		}
	})

	Context("when no entry exist for route", func() {
		BeforeEach(func() {
			routingTable = routing_table.NewTable(logger, nil)
			modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 0}
		})

		Describe("SetRoutes", func() {
			It("emits nothing", func() {

				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.SetRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("AddEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("RemoveEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})
		})
	})

	Context("when there exists an entry for route", func() {
		var (
			endpoints        map[routing_table.EndpointKey]routing_table.Endpoint
			key              routing_table.RoutingKey
			logGuid          string
			externalEndpoint routing_table.ExternalEndpointInfos
		)

		BeforeEach(func() {
			logGuid = "log-guid-1"
			externalEndpoint = routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo(61000),
			}
			key = routing_table.NewRoutingKey("process-guid-1", 5222)
			modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 1}
			endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}
		})

		Describe("SetRoutes", func() {
			BeforeEach(func() {
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			Context("only external port changes", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61001,
							ContainerPort: 5222,
						},
					}
				})

				It("emits routing event", func() {
					modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents := routingTable.SetRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					externalInfo := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo(61000),
						routing_table.NewExternalEndpointInfo(61001),
					}
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.SetRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("multiple external ports are specified", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61000,
							ContainerPort: 5222,
						},
						tcp_routes.TCPRoute{
							ExternalPort:  61001,
							ContainerPort: 5222,
						},
						tcp_routes.TCPRoute{
							ExternalPort:  61002,
							ContainerPort: 5222,
						},
					}
				})

				It("emits routing event", func() {
					modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)

					routingEvents := routingTable.SetRoutes(desiredLRP)

					externalInfo := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo(61000),
						routing_table.NewExternalEndpointInfo(61001),
						routing_table.NewExternalEndpointInfo(61002),
					}
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, modificationTag)

					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("no changes to external port", func() {
				It("emits nothing", func() {
					tag := receptor.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, tag)
					routingEvents := routingTable.SetRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("when two disjoint (external port, container port) pairs are given", func() {
				var endpoints2 map[routing_table.EndpointKey]routing_table.Endpoint
				var key2 routing_table.RoutingKey

				BeforeEach(func() {
					key2 = routing_table.NewRoutingKey("process-guid-1", 5223)
					endpoints2 = map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 63004, 5223, modificationTag),
					}
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key:  routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
						key2: routing_table.NewRoutableEndpoints(nil, endpoints2, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(2))

					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61001,
							ContainerPort: 5222,
						},
						tcp_routes.TCPRoute{
							ExternalPort:  61002,
							ContainerPort: 5223,
						},
					}
				})

				It("emits two separate registration events with no overlap", func() {
					modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)

					externalInfo1 := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo(61000),
						routing_table.NewExternalEndpointInfo(61001),
					}
					expectedEntry1 := routing_table.NewRoutableEndpoints(
						externalInfo1, endpoints, logGuid, modificationTag)

					externalInfo2 := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo(61002),
					}
					expectedEntry2 := routing_table.NewRoutableEndpoints(
						externalInfo2, endpoints2, logGuid, modificationTag)

					expectMap := map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key:  expectedEntry1,
						key2: expectedEntry2,
					}

					routingEvents := routingTable.SetRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(2))
					Expect(routingEvents[0].Entry).Should(Equal(expectMap[routingEvents[0].Key]))
					Expect(routingEvents[1].Entry).Should(Equal(expectMap[routingEvents[1].Key]))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})

			Context("when container ports doesn't match", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61001,
							ContainerPort: 5223,
						},
					}
				})

				It("emits nothing", func() {
					newTag := receptor.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newTag)
					routingEvents := routingTable.SetRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})
		})

		Describe("AddEndpoint", func() {

			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoint, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits routing events", func() {
					newTag := receptor.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.AddEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

					expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 61104, 5222, newTag),
					}

					expectedEntry := routing_table.NewRoutableEndpoints(
						externalEndpoint, expectedEndpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with different instance guid", func() {
					It("emits routing events", func() {
						newTag := receptor.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 61104, 5222, false, newTag)
						routingEvents := routingTable.AddEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(1))
						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

						expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{}
						for k, v := range endpoints {
							expectedEndpoints[k] = v
						}
						expectedEndpoints[routing_table.NewEndpointKey("instance-guid-3", false)] =
							routing_table.NewEndpoint(
								"instance-guid-3", false, "some-ip-3", 61104, 5222, newTag)
						expectedEntry := routing_table.NewRoutableEndpoints(
							externalEndpoint, expectedEndpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry.Endpoints).Should(HaveLen(3))
						Expect(routingEvent.Entry).Should(Equal(expectedEntry))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := receptor.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, newTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 61105, 5222, newTag),
								routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
									"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoint, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(2))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := receptor.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, olderTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})

		Describe("RemoveEndpoint", func() {

			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoint, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits nothing", func() {
					newTag := receptor.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.RemoveEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(0))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with instance guid not present in existing endpoints", func() {
					It("emits nothing", func() {
						newTag := receptor.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 62004, 5222, false, newTag)
						routingEvents := routingTable.RemoveEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := receptor.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, newTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
									"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoint, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("same modification tag", func() {
						It("emits routing events", func() {
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, modificationTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
									"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoint, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := receptor.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, olderTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})
	})
})
