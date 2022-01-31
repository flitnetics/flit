package main

import (
        mqtt "github.com/eclipse/paho.mqtt.golang"
        "time"
        "fmt"
        "github.com/google/uuid"

        "os"
        "github.com/jinzhu/configor"
        "os/user"
        "encoding/json"
)

var Config = struct {
        Organization string `required:"true"`
	Broker       string `required:"true"`
	Port         int    `reqiured:"True"`
}{}

var res map[string]interface{}

func publish(client mqtt.Client, body []byte, remoteClient string, org string) {
        text := string(body)
        fmt.Println(fmt.Sprintf("Publishing %s on topic %s/%s", string(body), org, remoteClient))
        token := client.Publish(fmt.Sprintf("%s/%s", org, remoteClient), 0, false, text)
        token.Wait()
        time.Sleep(time.Second)
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
    fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    fmt.Printf("Connect lost: %v", err)
}

func sendCommands() ([]byte, string) {
        // arguments
        action := os.Args[1]
        remoteClient := os.Args[2]

        if (action == "" || remoteClient == "") {
               fmt.Println("Need action, such as 'update'  and Location Id")
        } else if (action == "update" && remoteClient != "") {

               mapD := map[string]string{"action": "update", "locationId": remoteClient}
               body, _ := json.Marshal(mapD)

               return body, remoteClient
        } else if (action == "ping" && remoteClient != "") {
               mapD := map[string]string{"action": "ping", "locationId": remoteClient}
               body, _ := json.Marshal(mapD)

               return body, remoteClient
        }

        return nil, remoteClient
}

func main() {
    usr, err := user.Current()
    if err != nil {
        panic(err)
    }

    if err := configor.Load(&Config, fmt.Sprintf("%s/.flit/config.yaml", usr.HomeDir)); err != nil {
            panic(err)
    }

    // topic organization
    organization := Config.Organization
    // broker hostname
    broker := Config.Broker
    // broker port (eg. 1883)
    port := Config.Port

    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
    opts.SetClientID(uuid.New().String())
    opts.SetUsername("emqx")
    opts.SetPassword("public")
    opts.OnConnect = connectHandler
    opts.OnConnectionLost = connectLostHandler
    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }
    
    body, remoteClient := sendCommands()
    publish(client, body, remoteClient, organization)

    client.Disconnect(250)
}
