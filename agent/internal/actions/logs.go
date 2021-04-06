package actions

import (
        "context"
        "io/ioutil"

        "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func Logs() ([]byte, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	options := types.ContainerLogsOptions{ShowStdout: true}
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

        var logs []byte
	for _, container := range containers {
                out, err := cli.ContainerLogs(ctx, container.ID, options)
                if err != nil {
                        panic(err)
                }
                logs, err = ioutil.ReadAll(out)
                if err != nil {
                        panic(err)
                }
	}

        return logs, nil
}

