package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"
	configuration "github.com/himalayo/clusterfuck/api/configuration"
	cfgpb "github.com/himalayo/clusterfuck/api/configuration/proto"
	events "github.com/himalayo/clusterfuck/api/events"
	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"google.golang.org/protobuf/proto"

	player "github.com/himalayo/clusterfuck/api/player"
	"github.com/redis/go-redis/v9"
)

var (
	addr              = flag.String("addr", ":2096", "http service address")
	playerAddr        = flag.String("player_addr", "localhost:50053", "player service address")
	configurationAddr = flag.String("cfg_addr", "localhost:50057", "configuration gRPC API address")
	Man               = newManager()
	Player, _         = player.NewClient("networking-service", &redis.Options{
		Addr:     os.Getenv("PLAYER_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("PLAYER_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	})
	Cfg  = configuration.NewClient()
	Auth = newAuth()
)

type Set struct {
	data map[string]bool
	mu   sync.Mutex
}

func NewSet() *Set {
	return &Set{
		data: make(map[string]bool),
	}
}

func (s *Set) IsElement(x string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[x]
	return ok
}

func (s *Set) SetElement(x string) {
	s.mu.Lock()
	s.data[x] = true
	s.mu.Unlock()
}

var (
	OutgoingServices = NewSet()
	IncomingServices = NewSet()
)

func RegisterIncomingService(service *cfgpb.IncomingService) {
	log.Printf("Registering to Stream %s on Address %s", service.Stream.Stream, service.Stream.Instance.Address)
	if IncomingServices.IsElement(fmt.Sprintf("%s-%s", service.Stream.Stream, service.Stream.Instance.Address)) {
		log.Printf("Already publishing to Stream %s on Address %s", service.Stream.Stream, service.Stream.Instance.Address)
		return
	}
	IncomingServices.SetElement(fmt.Sprintf("%s-%s", service.Stream.Stream, service.Stream.Instance.Address))
	pub := events.NewRedisPublisher(&redis.Options{
		Addr:     service.Stream.Instance.GetAddress(),
		Password: service.Stream.Instance.GetPassword(),
		DB:       int(service.Stream.Instance.GetDb()),
	}, service.Stream.Stream)

	headers := service.GetHeaders()
	for _, header := range headers {
		log.Printf("Registering header %d to %s", header, service.Stream.Instance.GetAddress())
		RegisterHandler(int16(header), func(c *Client, _ []byte, packet []byte) {
			go func() {
				id_uuid, err := uuid.NewRandom()
				var id string
				if err != nil {
					id = fmt.Sprintf("%v", rand.Float64())
				} else {
					id = id_uuid.String()
				}
				value := make(map[string]interface{})
				value["id"] = id
				value["type"] = fmt.Sprintf("%d", header)
				value["packet"] = packet
				value["client_id"] = c.sso
				log.Printf("Publishing %d to %s (%s)", header, pub.Stream, service.Stream.Instance.GetAddress())
				pub.Publish(context.Background(), &PacketEvent{
					Id:     id,
					Type:   fmt.Sprintf("%d", header),
					Value:  value,
					Packet: packet,
				})
			}()
		})
	}

}

func RegisterOutgoingService(service *cfgpb.OutgoingService) {
	log.Printf("Registering to Pub/Sub %s on Address %s", "network-pubsub-outgoing", service.Instance.Address)
	if OutgoingServices.IsElement(service.Instance.Address) {
		return
	}
	OutgoingServices.SetElement(service.Instance.Address)

	redis_client := redis.NewClient(&redis.Options{
		Addr:     service.Instance.Address,
		Password: service.Instance.Password,
		DB:       int(service.Instance.GetDb()),
	})

	go func() {
		pubsub := redis_client.Subscribe(context.Background(), "network-pubsub-outgoing")
		ch := pubsub.Channel()
		for msg := range ch {
			data := []byte(msg.Payload)
			var evt pb.PacketEvent
			err := proto.Unmarshal(data, &evt)
			if err == nil {
				client, ok := Man.sessions[evt.Packet.ClientId]
				if client != nil && ok {
					parsed := ParseMessage(evt.Packet.Packet)
					log.Printf("Sending from Pub/Sub %s to %s: %d", service.Instance.Address, evt.Packet.ClientId, parsed.header)
					client.send <- evt.Packet.Packet
				}
			} else {
				log.Printf("Could not read from Pub/Sub %s: %v", service.Instance.Address, err)
			}
		}
	}()
}

func HandleNetworkingTopology(topology *cfgpb.NetworkingConfiguration) {
	log.Printf("Handling networking topology")
	go func() {
		for _, service := range topology.Incoming {
			RegisterIncomingService(service)
		}
	}()

	go func() {
		for _, service := range topology.Outgoing {
			RegisterOutgoingService(service)
		}
	}()
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	plAddr, present := os.LookupEnv("PLAYER_HOST")
	if !present {
		plAddr = *playerAddr
	}

	cfgAddr, present := os.LookupEnv("CONFIGURATION_HOST")
	if !present {
		cfgAddr = *configurationAddr
	}
	go Man.run()
	go Player.Listen(plAddr)
	go Auth.run(Player, Man)
	RegisterHandlers()
	go StartRpc()
	go func() {
		Cfg.Listen(cfgAddr)
		group_uuid, err := uuid.NewRandom()
		var group string
		if err != nil {
			group = fmt.Sprintf("%v", rand.Float64())
		} else {
			group = group_uuid.String()
		}
		network_topology, err := Cfg.GetNetworkingConfiguration(context.Background(), group)
		if err == nil {
			HandleNetworkingTopology(network_topology)
			Cfg.RegisterNetworkingConfigurationEventHandler(configuration.NetworkingRegisterEventType, func(ctx context.Context, evt *cfgpb.NetworkingConfigurationEvent) {
				if evt.GetType() != configuration.NetworkingRegisterEventType {
					return
				}
				go RegisterIncomingService(evt.Incoming)
				go RegisterOutgoingService(evt.Outgoing)
			})
			Cfg.RegisterNetworkingConfigurationEventHandler(configuration.NetworkingUpdateEventType, func(ctx context.Context, evt *cfgpb.NetworkingConfigurationEvent) {
				if evt.GetType() != configuration.NetworkingUpdateEventType {
					return
				}
				go RegisterIncomingService(evt.Incoming)
				go RegisterOutgoingService(evt.Outgoing)
			})
		} else {
			log.Printf("Got error fetching networking topology: %v", err)
		}
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(Man, w, r)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func init() {
	initResolving()
}
