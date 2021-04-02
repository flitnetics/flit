package mocks

import (
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/docker/docker/api/types"
	container2 "github.com/docker/docker/api/types/container"
	"time"
)

// CreateMockContainer creates a container substitute valid for testing
func CreateMockContainer(id string, name string, image string, created time.Time) container.Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Image:   image,
			Name:    name,
			Created: created.String(),
		},
		Config: &container2.Config{
			Image:  image,
			Labels: make(map[string]string),
		},
	}
	return *container.NewContainer(
		&content,
		&types.ImageInspect{
			ID: image,
			RepoDigests: []string{
				image,
			},
		},
	)
}

// CreateMockContainerWithImageInfo should only be used for testing
func CreateMockContainerWithImageInfo(id string, name string, image string, created time.Time, imageInfo types.ImageInspect) container.Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Image:   image,
			Name:    name,
			Created: created.String(),
		},
		Config: &container2.Config{
			Image:  image,
			Labels: make(map[string]string),
		},
	}
	return *container.NewContainer(
		&content,
		&imageInfo,
	)
}

// CreateMockContainerWithDigest should only be used for testing
func CreateMockContainerWithDigest(id string, name string, image string, created time.Time, digest string) container.Container {
	c := CreateMockContainer(id, name, image, created)
	c.ImageInfo().RepoDigests = []string{digest}
	return c
}

// CreateMockContainerWithConfig creates a container substitute valid for testing
func CreateMockContainerWithConfig(id string, name string, image string, created time.Time, config *container2.Config) container.Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Image:   image,
			Name:    name,
			Created: created.String(),
		},
		Config: config,
	}
	return *container.NewContainer(
		&content,
		&types.ImageInspect{
			ID: image,
		},
	)
}
