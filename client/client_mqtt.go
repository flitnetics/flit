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
        Master_Url string `required:"true"`
        Organization string `required:"true"`
}{}

var res map[string]interface{}

func publish(client mqtt.Client, body []byte, remoteClient string, org string) {
    num := 10
    for i := 0; i < num; i++ {
        //text := fmt.Sprintf("Hello Zaihan Number %d", i)
        text := string(body)
        fmt.Println("Publishing!")
        token := client.Publish(fmt.Sprintf("%s/%s", org, remoteClient), 0, false, text)
        token.Wait()
        time.Sleep(time.Second)
    }
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
    fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
    fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    fmt.Printf("Connect lost: %v", err)
}

func mainLogic() ([]byte, string, string) {
        usr, err := user.Current()
        if err != nil {
            panic(err)
        }

        if err := configor.Load(&Config, fmt.Sprintf("%s/.hub/config.yaml", usr.HomeDir)); err != nil {
                panic(err)
        }

        // topic organization
        organization := Config.Organization

        // arguments
        action := os.Args[1]
        remoteClient := os.Args[2]

        if (action == "" || remoteClient == "") {
               fmt.Println("Need action, such as 'update'  and Location Id")
        } else if (action == "update" && remoteClient != "") {

               mapD := map[string]string{"action": "update", "locationId": remoteClient}
               body, _ := json.Marshal(mapD)

               return body, remoteClient, organization
        } else if (action == "ping" && remoteClient != "") {
               mapD := map[string]string{"action": "ping", "locationId": remoteClient}
               body, _ := json.Marshal(mapD)

               return body, remoteClient, organization
        }

        return nil, remoteClient, organization
}

func main() {
    var broker = "broker.emqx.io"
    var port = 1883
    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
    opts.SetClientID(uuid.New().String())
//    opts.SetDefaultPublishHandler(messagePubHandler)
    opts.SetUsername("emqx")
    opts.SetPassword("public")
    opts.OnConnect = connectHandler
    opts.OnConnectionLost = connectLostHandler
    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }

    body, remoteClient, org := mainLogic()
    publish(client, body, remoteClient, org)

    client.Disconnect(250)
}
