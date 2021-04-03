package actions

import (
        "context"

        "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Image struct {
  Name string `json:"name,omitempty"`
  Hash string `json:"hash,omitempty"` 
}

func List() ([]Image, error) {
        ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

        containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
        if err != nil {
                panic(err)
        }

        var images []Image
        for _, container := range containers {
                image := Image{Name: container.Image, Hash: container.ImageID}
                images = append(images, image)
        }

        return images, nil
}

