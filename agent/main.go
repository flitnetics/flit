package main

import (
        "os"
        "os/signal"
        "syscall"
        "time"
        "encoding/json"
        "bytes"
        "net/http"
        "fmt"
        "log"

        "github.com/gojektech/heimdall/v6/httpclient"

        "github.com/jinzhu/configor"

        "github.com/flitnetics/flit/agent/internal/actions"

        mqtt "github.com/eclipse/paho.mqtt.golang"
        "github.com/google/uuid"
)

var Config = struct {
        Client struct {
                Location_Id     string `default:"all"`
                Ui_Url          string `default:"localhost"`
                Enable_Ui       bool   `default:"false"`
                Organization    string `required:"true"`
                Mqtt struct {
                        Broker          string  `required:"true"`
                        Port            int  `required:"true"`
                        Username        string
                        Password        string
                }
        }
}{}

type Payload struct {
      Action string `json:"action"`
      LocationId string `json:"locationId"`
      Image []actions.Image `json:"images,omitempty"`
      ImageError string `json:"error,omitempty"`
      LoggingInfo []actions.LoggingInfo `json:"logs,omitempty"`
}

type Agent struct {
   Payload Payload `json:"agent"`
}

func runList() []actions.Image {
        containers, err := actions.List()
        if err != nil {
                 log.Println(err)
        }

        return containers
}

func runLogs() []actions.LoggingInfo {
        containers, err := actions.Logs()
        if err != nil {
                 log.Println(err)
        }

        return containers
}


func listenMqtt() {
    if err := configor.Load(&Config, "config.yaml"); err != nil {
            panic(err)
    }

    configor.Load(&Config, "config.yaml")

    fmt.Println(fmt.Sprintf("broker: %s", Config.Client.Mqtt.Broker))
    fmt.Println(fmt.Sprintf("port: %d", Config.Client.Mqtt.Port))

    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://%s:%d", Config.Client.Mqtt.Broker, Config.Client.Mqtt.Port))
    opts.SetClientID(uuid.New().String())
    opts.SetDefaultPublishHandler(messagePubHandler) // needed for subscriber
    opts.SetUsername(Config.Client.Mqtt.Username)
    opts.SetPassword(Config.Client.Mqtt.Password)
    opts.OnConnect = connectHandler
    opts.OnConnectionLost = connectLostHandler
    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())

    }

    // subscribe (listen) to MQTT
    sub(client)
}

func sendToUI(json []byte, endpoint string) {
                timeout := 1000 * time.Millisecond
                client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

                headers := http.Header{}
                headers.Set("Content-Type", "application/json")
                body := bytes.NewReader([]byte(string(json)))

                // Use the clients GET method to create and execute the request
                _, err := client.Post(fmt.Sprintf("%s/api/v1/agents/%s", Config.Client.Ui_Url, endpoint), body, headers)
                if err != nil{
                       log.Println(err)
                }

}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
    fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

/* func messagePubHandler(filter t.Filter) func(mqtt.Client, mqtt.Message) {
    //fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

    return func(client mqtt.Client, msg mqtt.Message) {
            processMsg(msg, filter)
    }
} */

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
    fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    fmt.Printf("Connect lost: %v", err)
}

// we process what we need to do, updates or ping
func processMsg(msg mqtt.Message) {
    if err := configor.Load(&Config, "config.yaml"); err != nil {
            panic(err)
    }

    configor.Load(&Config, "config.yaml")

    message := string(msg.Payload())
    // location Id
    location := Config.Client.Location_Id
    // Web UI Config
    ui := Config.Client.Enable_Ui

    payload := Payload{}
    json.Unmarshal([]byte(message), &payload)

    // "ping" action
    if ((payload.LocationId == location || location == "all") && payload.Action == "ping") {
            images := runList()
            logs := runLogs()

            var mapD Agent
            var mapB []byte
            mapD = Agent{Payload: Payload{Action: "pong", LocationId: location, ImageError: "Containers are not running"}}
            mapB, _ = json.Marshal(mapD)

            // if there are no container images
            if (images != nil) {
                    mapD = Agent{Payload: Payload{Action: "pong", LocationId: location, Image: images, LoggingInfo: logs}}
                    mapB, _ = json.Marshal(mapD)
            }

            // if we enable the UI
            if (ui == true) {
                   sendToUI(mapB, "pong")
            }
    }

    log.Println(payload.Action)
}

func sub(client mqtt.Client) {
    configor.Load(&Config, "config.yaml")

    topic := fmt.Sprintf("%s/%s", Config.Client.Organization, Config.Client.Location_Id)
    token := client.Subscribe(topic, 1, nil)
    token.Wait()
    fmt.Printf("Subscribed to topic: %s\n", topic)
}

func main() {
    keepAlive := make(chan os.Signal)
    signal.Notify(keepAlive, os.Interrupt, syscall.SIGTERM)

    // listen for MQTT traffic
    listenMqtt()

    <-keepAlive
}
