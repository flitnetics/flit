package main

import (
  "log"
  "context"
  "encoding/json"
  "time"
  "fmt"
  "github.com/r3labs/sse/v2"
  "net/http"
  "goji.io"
  "goji.io/pat"
  "github.com/go-redis/redis/v8"
)

/*
func sendMessage(server *sse.Server) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {

    name := pat.Param(r, "name")
    log.Println("sendMessage function %s", name)

    server.Publish("messages", &sse.Event{
      Data: []byte(name),
    })
  }
} */

func sendPubSub(w http.ResponseWriter, r *http.Request) {
        data := pat.Param(r, "name")

        redisClient := redis.NewClient(&redis.Options{
                Addr:     "localhost:6379",  // We connect to host redis, thats what the hostname of the redis service is set to in the docker-compose
                Password: "", // The password IF set in the redis Config file
                DB:       0,
        })

        err := redisClient.Ping(context.Background()).Err()
        if err != nil {
                // Sleep for 3 seconds and wait for Redis to initialize
                time.Sleep(3 * time.Second)
                err := redisClient.Ping(context.Background()).Err()
                if err != nil {
                        panic(err)
                }
        }
        // Generate a new background context that  we will use
        ctx := context.Background()

        redisClient.Publish(ctx, "new_users", data).Err()
}

func sendMessage2(server *sse.Server, data string) {
    log.Println("sendMessage2 function %s", data)

    server.Publish("messages", &sse.Event{
      Data: []byte(data),
    })
}

func main() {
  server := sse.New()
  server.CreateStream("messages")
  server.EncodeBase64 = true

  mux := goji.NewMux()
  mux.HandleFunc(pat.Get("/events"), server.HTTPHandler)
  mux.HandleFunc(pat.Get("/send/:name"), sendPubSub)

  go func() {
	// Create a new Redis Client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",  // We connect to host redis, thats what the hostname of the redis service is set to in the docker-compose
		Password: "", // The password IF set in the redis Config file
		DB:       0,
	})
	// Ping the Redis server and check if any errors occured
	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		// Sleep for 3 seconds and wait for Redis to initialize
		time.Sleep(3 * time.Second)
		err := redisClient.Ping(context.Background()).Err()
		if err != nil {
			panic(err)
		}
	}

	ctx := context.Background()
	// Subscribe to the Topic given
	topic := redisClient.Subscribe(ctx, "new_users")
	// Get the Channel to use
	channel := topic.Channel()
	// Itterate any messages sent on the channel
	for msg := range channel {
		/* u := &User{}
		// Unmarshal the data into the user
		err := u.UnmarshalBinary([]byte(msg.Payload))
		if err != nil {
			panic(err)
		} */

		fmt.Println(string(msg.Payload)) 
                sendMessage2(server, string(msg.Payload))
	}
  }()

  log.Print("Started server")
  http.ListenAndServe(":8080", mux)
}

// User is a struct representing newly registered users
type User struct {
	Username string
	Email    string
}

// MarshalBinary encodes the struct into a binary blob
// Here I cheat and use regular json :)
func (u *User) MarshalBinary() ([]byte, error) {
	return json.Marshal(u)
}

// UnmarshalBinary decodes the struct into a User
func (u *User) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, u); err != nil {
		return err
	}
	return nil
}

