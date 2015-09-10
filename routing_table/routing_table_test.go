package routing_table_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testRoutingTable struct {
	entries map[routing_table.RoutingKey]routing_table.RoutableEndpoints
}

func (t *testRoutingTable) RouteCount() int {
	return 0
}

func (t *testRoutingTable) SetRoutes(desiredLRP *models.DesiredLRP) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) AddEndpoint(actualLRP *models.ActualLRPGroup) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}
func (t *testRoutingTable) RemoveEndpoint(actualLRP *models.ActualLRPGroup) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) Swap(table routing_table.RoutingTable) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) GetRoutingEvents() routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

var _ = Describe("RoutingTable", func() {

	var (
		routingTable    routing_table.RoutingTable
		modificationTag *models.ModificationTag
		tcpRoutes       tcp_routes.TCPRoutes
	)

	getDesiredLRP := func(processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes, modificationTag *models.ModificationTag) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		portMap := map[uint32]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint32{}
		for k, _ := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint32, evacuating bool,
		modificationTag *models.ModificationTag) *models.ActualLRPGroup {
		if evacuating {
			return &models.ActualLRPGroup{
				Instance: nil,
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
			}
		} else {
			return &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
				Evacuating: nil,
			}
		}
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
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 0}
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

		Describe("Swap", func() {
			var (
				tempRoutingTable routing_table.RoutingTable
				key              routing_table.RoutingKey
				endpoints        map[routing_table.EndpointKey]routing_table.Endpoint
				modificationTag  *models.ModificationTag
				logGuid          string
			)

			BeforeEach(func() {
				logGuid = "log-guid-1"
				externalEndpoint := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo(61000),
				}
				key = routing_table.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
					routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}

				tempRoutingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
				})
			})

			It("emits routing events for new routes", func() {
				Expect(routingTable.RouteCount()).Should(Equal(0))
				routingEvents := routingTable.Swap(tempRoutingTable)
				Expect(routingTable.RouteCount()).Should(Equal(1))
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
				externalInfo := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo(61000),
				}
				expectedEntry := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
			})
		})

		Describe("GetRoutingEvents", func() {
			It("returns empty routing events", func() {
				routingEvents := routingTable.GetRoutingEvents()
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
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
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
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
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
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
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
					tag := &models.ModificationTag{Epoch: "abc", Index: 2}
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
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
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
					newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
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
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
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
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
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
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
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
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
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
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
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
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 62004, 5222, false, newTag)
						routingEvents := routingTable.RemoveEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
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
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, olderTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})

		Describe("GetRoutingEvents", func() {
			BeforeEach(func() {
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			It("returns routing events for entries in routing table", func() {
				routingEvents := routingTable.GetRoutingEvents()
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
				externalInfo := []routing_table.ExternalEndpointInfo{
					routing_table.NewExternalEndpointInfo(61000),
				}
				expectedEntry := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable routing_table.RoutingTable
				key              routing_table.RoutingKey
				endpoints        map[routing_table.EndpointKey]routing_table.Endpoint
				modificationTag  *models.ModificationTag
				logGuid          string
			)

			BeforeEach(func() {
				existingLogGuid := "log-guid-1"
				existingExternalEndpoint := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo(61000),
				}
				existingKey := routing_table.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				existingEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
					routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					existingKey: routing_table.NewRoutableEndpoints(existingExternalEndpoint, existingEndpoints, existingLogGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))

				logGuid = "log-guid-2"
				externalEndpoint := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo(62000),
				}
				key = routing_table.NewRoutingKey("process-guid-2", 6379)
				endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
					routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
						"instance-guid-1", false, "some-ip-3", 63004, 6379, modificationTag),
					routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
						"instance-guid-2", false, "some-ip-4", 63004, 6379, modificationTag),
				}
				tempRoutingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
				})
				Expect(tempRoutingTable.RouteCount()).Should(Equal(1))
			})

			It("overwrites the existing entries and emits routing events for new routes", func() {
				routingEvents := routingTable.Swap(tempRoutingTable)
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
				externalInfo := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo(62000),
				}
				expectedEntry := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})
		})
	})

	Describe("Swap", func() {

		var (
			tempRoutingTable testRoutingTable
		)

		BeforeEach(func() {
			routingTable = routing_table.NewTable(logger, nil)
			tempRoutingTable = testRoutingTable{}

			logGuid := "log-guid-1"
			externalEndpoint := routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo(61000),
			}
			key := routing_table.NewRoutingKey("process-guid-1", 5222)
			modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}

			tempRoutingTable.entries = map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
				key: routing_table.NewRoutableEndpoints(externalEndpoint, endpoints, logGuid, modificationTag),
			}

		})
		Context("when the routing tables are of different type", func() {

			It("should not swap the tables", func() {
				routingEvents := routingTable.Swap(&tempRoutingTable)
				Expect(routingEvents).To(HaveLen(0))
				Expect(routingTable.RouteCount()).Should(Equal(0))
			})
		})
	})

})
