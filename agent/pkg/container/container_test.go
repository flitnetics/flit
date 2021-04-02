package container

import (
	"testing"

	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	cli "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Container Suite")
}

var _ = Describe("the container", func() {
	Describe("the client", func() {
		var docker *cli.Client
		var client Client
		BeforeSuite(func() {
			server := mocks.NewMockAPIServer()
			docker, _ = cli.NewClientWithOpts(
				cli.WithHost(server.URL),
				cli.WithHTTPClient(server.Client()))
			client = dockerClient{
				api:        docker,
				pullImages: false,
			}
		})
		It("should return a client for the api", func() {
			Expect(client).NotTo(BeNil())
		})
		When("listing containers without any filter", func() {
			It("should return all available containers", func() {
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 2).To(BeTrue())
			})
		})
		When("listing containers with a filter matching nothing", func() {
			It("should return an empty array", func() {
				filter := filters.FilterByNames([]string{"lollercoaster"}, filters.NoFilter)
				containers, err := client.ListContainers(filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 0).To(BeTrue())
			})
		})
		When("listing containers with a watchtower filter", func() {
			It("should return only the watchtower container", func() {
				containers, err := client.ListContainers(filters.WatchtowerContainersFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 1).To(BeTrue())
				Expect(containers[0].ImageName()).To(Equal("containrrr/watchtower:latest"))
			})
		})
		When(`listing containers with the "include stopped" option`, func() {
			It("should return both stopped and running containers", func() {
				client = dockerClient{
					api:            docker,
					pullImages:     false,
					includeStopped: true,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) > 0).To(BeTrue())
			})
		})
		When(`listing containers with the "include restart" option`, func() {
			It("should return both stopped, restarting and running containers", func() {
				client = dockerClient{
					api:               docker,
					pullImages:        false,
					includeRestarting: true,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				RestartingContainerFound := false
				for _, ContainerRunning := range containers {
					if ContainerRunning.containerInfo.State.Restarting {
						RestartingContainerFound = true
					}
				}
				Expect(RestartingContainerFound).To(BeTrue())
				Expect(RestartingContainerFound).NotTo(BeFalse())
			})
		})
		When(`listing containers without restarting ones`, func() {
			It("should not return restarting containers", func() {
				client = dockerClient{
					api:               docker,
					pullImages:        false,
					includeRestarting: false,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				RestartingContainerFound := false
				for _, ContainerRunning := range containers {
					if ContainerRunning.containerInfo.State.Restarting {
						RestartingContainerFound = true
					}
				}
				Expect(RestartingContainerFound).To(BeFalse())
				Expect(RestartingContainerFound).NotTo(BeTrue())
			})
		})
	})
	When("asked for metadata", func() {
		var c *Container
		BeforeEach(func() {
			c = mockContainerWithLabels(map[string]string{
				"com.centurylinklabs.watchtower.enable": "true",
				"com.centurylinklabs.watchtower":        "true",
			})
		})
		It("should return its name on calls to .Name()", func() {
			name := c.Name()
			Expect(name).To(Equal("test-containrrr"))
			Expect(name).NotTo(Equal("wrong-name"))
		})
		It("should return its ID on calls to .ID()", func() {
			id := c.ID()

			Expect(id).To(Equal("container_id"))
			Expect(id).NotTo(Equal("wrong-id"))
		})
		It("should return true, true if enabled on calls to .Enabled()", func() {
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeTrue())
			Expect(enabled).NotTo(BeFalse())
			Expect(exists).To(BeTrue())
			Expect(exists).NotTo(BeFalse())
		})
		It("should return false, true if present but not true on calls to .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower.enable": "false"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeTrue())
			Expect(exists).NotTo(BeFalse())
		})
		It("should return false, false if not present on calls to .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"lol": "false"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeFalse())
			Expect(exists).NotTo(BeTrue())
		})
		It("should return false, false if present but not parsable .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower.enable": "falsy"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeFalse())
			Expect(exists).NotTo(BeTrue())
		})
		When("checking if its a watchtower instance", func() {
			It("should return true if the label is set to true", func() {
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeTrue())
			})
			It("should return false if the label is present but set to false", func() {
				c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower": "false"})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
			It("should return false if the label is not present", func() {
				c = mockContainerWithLabels(map[string]string{"funny.label": "false"})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
			It("should return false if there are no labels", func() {
				c = mockContainerWithLabels(map[string]string{})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
		})
		When("fetching the custom stop signal", func() {
			It("should return the signal if its set", func() {
				c = mockContainerWithLabels(map[string]string{
					"com.centurylinklabs.watchtower.stop-signal": "SIGKILL",
				})
				stopSignal := c.StopSignal()
				Expect(stopSignal).To(Equal("SIGKILL"))
			})
			It("should return an empty string if its not set", func() {
				c = mockContainerWithLabels(map[string]string{})
				stopSignal := c.StopSignal()
				Expect(stopSignal).To(Equal(""))
			})
		})
		When("fetching the image name", func() {
			When("the zodiac label is present", func() {
				It("should fetch the image name from it", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.zodiac.original-image": "the-original-image",
					})
					imageName := c.ImageName()
					Expect(imageName).To(Equal(imageName))
				})
			})
			It("should return the image name", func() {
				name := "image-name:3"
				c = mockContainerWithImageName(name)
				imageName := c.ImageName()
				Expect(imageName).To(Equal(name))
			})
			It("should assume latest if no tag is supplied", func() {
				name := "image-name"
				c = mockContainerWithImageName(name)
				imageName := c.ImageName()
				Expect(imageName).To(Equal(name + ":latest"))
			})
		})

		When("fetching container links", func() {
			When("the depends on label is present", func() {
				It("should fetch depending containers from it", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("postgres"), HaveLen(1)))
				})
				It("should fetch depending containers if there are many", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres,redis",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("postgres"), ContainElement("redis"), HaveLen(2)))
				})
				It("should fetch depending containers if label is blank", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "",
					})
					links := c.Links()
					Expect(links).To(HaveLen(0))
				})
			})
			When("the depends on label is not present", func() {
				It("should fetch depending containers from host config links", func() {
					c = mockContainerWithLinks([]string{
						"redis:test-containrrr",
						"postgres:test-containrrr",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("redis"), ContainElement("postgres"), HaveLen(2)))
				})
			})
		})
	})
})

func mockContainerWithImageName(name string) *Container {
	container := mockContainerWithLabels(nil)
	container.containerInfo.Config.Image = name
	return container
}

func mockContainerWithLinks(links []string) *Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    "container_id",
			Image: "image",
			Name:  "test-containrrr",
			HostConfig: &container.HostConfig{
				Links: links,
			},
		},
		Config: &container.Config{
			Labels: map[string]string{},
		},
	}
	return NewContainer(&content, nil)
}

func mockContainerWithLabels(labels map[string]string) *Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    "container_id",
			Image: "image",
			Name:  "test-containrrr",
		},
		Config: &container.Config{
			Labels: labels,
		},
	}
	return NewContainer(&content, nil)
}
