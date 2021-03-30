package main

import (
  "log"
  "github.com/r3labs/sse/v2"
  "net/http"
  "goji.io"
  "goji.io/pat"
)

func sendMessage(server *sse.Server) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {

    name := pat.Param(r, "name")
    log.Println("sendMessage function %s", name)

    server.Publish("messages", &sse.Event{
      Data: []byte(name),
    })
  }
}

func main() {
  server := sse.New()
  server.CreateStream("messages")
  server.AutoReplay = true

  mux := goji.NewMux()
  mux.HandleFunc(pat.Get("/events"), server.HTTPHandler)
  mux.HandleFunc(pat.Get("/send/:name"), sendMessage(server))
  
  log.Print("Started server")
  http.ListenAndServe(":8080", mux)
}
