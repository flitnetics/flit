package main

import (
  "encoding/json"
  "bytes"
  "time"
  "github.com/gojektech/heimdall/v6/httpclient"
  "os"
  "fmt"
  "github.com/jinzhu/configor"
  "net/http"
  "os/user"
)

var Config = struct {
        Master_Url string `required:"true"`
}{}

func main() {
        timeout := 1000 * time.Millisecond
        client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

        usr, err := user.Current()
        if err != nil {
            panic(err)
        }

        if err := configor.Load(&Config, fmt.Sprintf("%s/.hub/config.yaml", usr.HomeDir)); err != nil {
                panic(err)
        }

        action := os.Args[1]
        remoteClient := os.Args[2]

        if (action == "" || remoteClient == "") {
               fmt.Println("Need action, such as 'update'  and Location Id")
        } else if (action == "update" && remoteClient != "") {
	
               mapD := map[string]string{"action": "update", "locationId": remoteClient}
               mapB, _ := json.Marshal(mapD)
               headers := http.Header{}
	       headers.Set("Content-Type", "application/json")
               body := bytes.NewReader([]byte(string(mapB)))

               // Use the clients GET method to create and execute the request
               _, err := client.Post(fmt.Sprintf("%s/update", Config.Master_Url), body, headers)
               if err != nil{
	               panic(err)
               }
        } else if (action == "ping" && remoteClient != "") {
               mapD := map[string]string{"action": "ping", "locationId": remoteClient}
               mapB, _ := json.Marshal(mapD)
               headers := http.Header{}
               headers.Set("Content-Type", "application/json")
               body := bytes.NewReader([]byte(string(mapB)))

               // Use the clients GET method to create and execute the request
               _, err := client.Post(fmt.Sprintf("%s/ping", Config.Master_Url), body, headers)
               if err != nil{
                       panic(err)
               }
        }
}
