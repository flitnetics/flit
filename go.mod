module github.com/muhammadn/docker-push

replace github.com/r3labs/sse/v2 v2.3.2 => github.com/muhammadn/sse/v2 v2.3.2

go 1.16

require (
	github.com/containrrr/watchtower v1.4.0
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/go-redis/redis/v8 v8.8.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/jinzhu/configor v1.2.1
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/r3labs/sse/v2 v2.3.2
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	goji.io v2.0.2+incompatible
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
)
