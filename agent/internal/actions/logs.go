package actions

import (
        "context"
        "io/ioutil"

        "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type LoggingInfo struct {
  Name string `json:"container_name"`
  Log string `json:"log"`
}

func Logs() ([]LoggingInfo, error) {
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

        var logs []LoggingInfo
	for _, container := range containers {
                out, err := cli.ContainerLogs(ctx, container.ID, options)
                if err != nil {
                        panic(err)
                }

                logData, err := ioutil.ReadAll(out)

                containerInfo := LoggingInfo{Name: container.Image, Log: string(logData)}
                logs = append(logs, containerInfo) // append as string
                if err != nil {
                        panic(err)
                }
                
	}

        return logs, nil
}

