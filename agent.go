package main

import (
  "github.com/r3labs/sse/v2"
  "fmt"
)

func main() {
  client := sse.NewClient("http://localhost:8080/events")

  client.Subscribe("messages", func(msg *sse.Event) {
    // Got some data!
    fmt.Println(string(msg.Data))
  }) 
}
