package main

import (
        "github.com/r3labs/sse/v2"
        "fmt"
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func main() {
  sseClient := sse.NewClient("http://localhost:8080/events")

  sseClient.Subscribe("messages", func(msg *sse.Event) {
    // Got some data!
    fmt.Println(string(msg.Data))

    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
      panic(err)
    }

    reader, err := cli.ImagePull(ctx, fmt.Sprintf("docker.io/library/%s", string(msg.Data)), types.ImagePullOptions{})
    if err != nil {
      panic(err)
    }
    io.Copy(os.Stdout, reader)

    resp, err := cli.ContainerCreate(ctx, &container.Config{
      Image: string(msg.Data),
      Cmd:   []string{"echo", "hello world"},
      Tty:   false,
    }, nil, nil, nil, "")
    if err != nil {
      panic(err)
    }

    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
      panic(err)
    }

    statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    select {
      case err := <-errCh:
	if err != nil {
          panic(err)
	}
      case <-statusCh:
    }

    out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
      if err != nil {
	panic(err)
    }
 
    stdcopy.StdCopy(os.Stdout, os.Stderr, out)
  }) 
}