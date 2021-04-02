package main

import (
  "time"
  "github.com/gojektech/heimdall/v6/httpclient"
  "os"
  "fmt"
  "github.com/jinzhu/configor"
)

var Config = struct {
        Master_Url string `required:"true"`
}{}

func main() {
        timeout := 1000 * time.Millisecond
        client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))
        if err := configor.Load(&Config, "config.yaml"); err != nil {
                panic(err)
        }

        action := os.Args[1]
        remoteClient := os.Args[2]

        if (action == "" || remoteClient == "") {
               fmt.Println("Need action, such as 'update'  and Location Id")
        } else if (action == "update" && remoteClient != "") {
               // Use the clients GET method to create and execute the request
                _, err := client.Get(fmt.Sprintf("%s/send/%s", Config.Master_Url, remoteClient), nil)
                // fmt.Println(fmt.Sprintf("%s/send/%s", Config.Master_Url, remoteClient))
                if err != nil{
	                panic(err)
                }
        }
}
