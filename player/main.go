package main

import (
	"context"
	"flag"
	"log"
	"os"

	achievements "github.com/himalayo/clusterfuck/api/achievements"
	configuration "github.com/himalayo/clusterfuck/api/configuration"
	modtool "github.com/himalayo/clusterfuck/api/modtool"
	networking "github.com/himalayo/clusterfuck/api/networking"
	listener "github.com/himalayo/clusterfuck/api/networking/listener/server"
	permission "github.com/himalayo/clusterfuck/api/permission"
	subscription "github.com/himalayo/clusterfuck/api/subscription"
	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
)

var (
	networkingAddr    = flag.String("net_addr", "localhost:50051", "networking gRPC API address")
	subscriptionAddr  = flag.String("sub_addr", "localhost:50052", "subscription gRPC API address")
	permissionAddr    = flag.String("per_addr", "localhost:50054", "permission gRPC API address")
	modtoolAddr       = flag.String("mod_addr", "localhost:50056", "modtool gRPC API address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	achievementsAddr  = flag.String("ach_addr", "localhost:50058", "achievements gRPC API address")
	Net               = networking.NewClient()
	Sub               = subscription.NewClient()
	Perm              = permission.NewClient()
	Mod               = modtool.NewClient()
	Cfg               = configuration.NewClient()
	Ach               = achievements.NewClient()
	Incoming          = listener.NewIncomingServer()
	Data              = NewDatabase(ConfigDatabaseFromEnv())
	events_cfg        = redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	}
	Events = NewEventListener(&events_cfg)
)
var NetPub *networking.NetworkingPublisher

func main() {
	flag.Parse()

	netAddr, present := os.LookupEnv("NETWORKING_HOST")
	if !present {
		netAddr = *networkingAddr
	}

	subAddr, present := os.LookupEnv("SUBSCRIPTION_HOST")
	if !present {
		subAddr = *subscriptionAddr
	}

	permAddr, present := os.LookupEnv("PERMISSION_HOST")
	if !present {
		permAddr = *permissionAddr
	}

	modAddr, present := os.LookupEnv("MODTOOL_HOST")
	if !present {
		modAddr = *modtoolAddr
	}

	confAddr, present := os.LookupEnv("CONFIGURATION_HOST")
	if !present {
		confAddr = *configurationAddr
	}

	achAddr, present := os.LookupEnv("ACHIEVEMENTS_HOST")
	if !present {
		achAddr = *achievementsAddr
	}
	NetPub = networking.NewNetworkingPublisher(&redis.Options{
		Addr:     os.Getenv("PLAYER_OUTGOING_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_OUTGOING_REDIS_PASSWORD"),
		DB:       0,
	})

	go func() {
		Cfg.Listen(confAddr)

		res, err := Cfg.RegisterService(context.Background(), "player-service", configuration.RedisStreamData{
			Stream: "networking-events",
			Group:  "player-service",
			Instance: &redis.Options{
				Addr:     events_cfg.Addr,
				Password: events_cfg.Password,
				DB:       events_cfg.DB,
			},
		}, []int{273, 357}, NetPub.RedisConfig)
		if err != nil {
			log.Printf("Got error while registering service: %v", err)
		}
		log.Printf("Success: %v Status: %v", res.Success, res.Status)
	}()

	go Net.Listen(netAddr)
	err := Net.SetRedisSubscriber(&redis.Options{
		Addr:     events_cfg.Addr,
		Password: events_cfg.Password,
		DB:       events_cfg.DB,
	}, "networking-events", "player-service")
	if err != nil {
		log.Printf("Got error setting Redis Subscriber for networking client: %v", err)
	}

	// Net.ConnectRedisHandler(
	// 	events_cfg.Addr,
	// 	events_cfg.Password,
	// 	events_cfg.DB,
	// 	"networking-events",
	// 	[]int{273},
	// )
	go Sub.Listen(subAddr)
	go Perm.Listen(permAddr)
	go Mod.Listen(modAddr)
	go Ach.Listen(achAddr)
	go Data.Listen()
	go Events.Listen()
	StartServer(Events)
}
