package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/himalayo/clusterfuck/api/events"
	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Packet struct {
	sso  string
	body []byte
}

type Packets struct {
	sso  string
	body [][]byte
}

type IncomingHandlerRequest struct {
	address     string
	headers     []int
	application string
}

type RedisConfiguration struct {
	address  string
	password string
	db       int
}

type RedisListenerRequest struct {
	RedisConfiguration
	stream  string
	headers []int
}

type NetworkingClient struct {
	send          chan Packet
	connect       chan IncomingHandlerRequest
	send_multiple chan Packets
	disconnect    chan IncomingHandlerRequest
	redis_connect chan RedisListenerRequest
	redis_sub     *events.RedisSubscriber
	Handle        chan PacketHandlerInstance
}

func NewClient() *NetworkingClient {
	return &NetworkingClient{
		send:          make(chan Packet),
		connect:       make(chan IncomingHandlerRequest),
		send_multiple: make(chan Packets),
		redis_connect: make(chan RedisListenerRequest),
		Handle:        make(chan PacketHandlerInstance),
	}
}

func sendPacket(client pb.NetworkingClient, msg *pb.Packet) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.SendPacket(ctx, msg)
	if err != nil {
		log.Printf("could not send packet: %v", err)
		return false
	}
	return succ.Successful
}

func sendPackets(client pb.NetworkingClient, msgs *pb.Packets) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.SendPackets(ctx, msgs)
	if err != nil {
		log.Printf("could not send packet: %v", err)
		return false
	}
	return succ.Successful
}

func connectHandler(client pb.NetworkingClient, req *pb.IncomingInstance) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.RegisterIncomingListener(ctx, req)
	if err != nil {
		log.Printf("could not connect incoming message handler (headers: %v): %v", req.Headers, err)
		return false
	}
	return succ.Successful
}

func disconnectHandler(client pb.NetworkingClient, req *pb.IncomingInstance) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.DisconnectIncomingListener(ctx, req)
	if err != nil {
		log.Printf("could not disconnect incoming message handler (headers: %v): %v", req.Headers, err)
		return false
	}
	return succ.Successful
}

func (c *NetworkingClient) connectRedisHandler(client pb.NetworkingClient, req *pb.RedisListener) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.RegisterRedisListener(ctx, req)
	if err != nil {
		log.Printf("could not disconnect incoming message handler (headers: %v): %v", req.Headers, err)
		return false
	}

	c.redis_sub, err = events.NewRedisSubscriber(&redis.Options{
		Addr:     req.Redis.GetAddress(),
		Password: req.Redis.GetPassword(),
		DB:       int(req.Redis.GetDb()),
	}, req.GetStream(), req.GetGroup())
	if err != nil {
		return false
	}
	go c.redis_sub.Listen(context.Background())
	return succ.Successful
}

func (n *NetworkingClient) ConnectHandler(address string, headers []int, application string) {
	n.connect <- IncomingHandlerRequest{address: address, headers: headers, application: application}
}

func (n *NetworkingClient) Send(sso string, data []byte) {
	n.send <- Packet{sso: sso, body: data}
}

func (n *NetworkingClient) SendMultiple(sso string, data [][]byte) {
	n.send_multiple <- Packets{sso: sso, body: data}
}

func (n *NetworkingClient) DisconnectHandler(address string, application string) {
	n.connect <- IncomingHandlerRequest{address: address, application: application}
}

func (n *NetworkingClient) ConnectRedisHandler(address string, password string, db int, stream string, headers []int) {
	n.redis_connect <- RedisListenerRequest{
		RedisConfiguration: RedisConfiguration{
			address:  address,
			password: password,
			db:       db,
		},
		headers: headers,
		stream:  stream,
	}
}

type PacketHandler func(context.Context, *pb.PacketEvent)
type PacketHandlerInstance struct {
	Header  int
	Handler PacketHandler
}

func (n *NetworkingClient) RegisterRedisHandler(header int, handler PacketHandler) {
	n.Handle <- PacketHandlerInstance{
		Header:  header,
		Handler: handler,
	}
}

func (n *NetworkingClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	c := pb.NewNetworkingClient(conn)
	for {
		select {
		case packet := <-n.send:
			go func() {
				sendPacket(c, &pb.Packet{ClientId: packet.sso, Packet: packet.body})
			}()

		case packets := <-n.send_multiple:
			go func() {
				sendPackets(c, &pb.Packets{ClientId: packets.sso, Packets: packets.body})
			}()

		case request := <-n.connect:
			go func() {
				headers := make([]int32, len(request.headers))
				for i := range request.headers {
					headers[i] = int32(request.headers[i])
				}
				connectHandler(c, &pb.IncomingInstance{Address: request.address, Headers: headers, Application: request.application})
			}()
		case request := <-n.disconnect:
			go func() {
				disconnectHandler(c, &pb.IncomingInstance{Address: request.address, Headers: nil, Application: request.application})
			}()
		case request := <-n.redis_connect:
			headers := make([]int32, len(request.headers))
			for i := range request.headers {
				headers[i] = int32(request.headers[i])
			}
			n.connectRedisHandler(c, &pb.RedisListener{
				Redis: &pb.RedisInstance{
					Address:  request.address,
					Password: request.password,
					Db:       int32(request.db),
				},
				Stream:  request.stream,
				Headers: headers,
			})
		case handler := <-n.Handle:
			n.redis_sub.Subscribe(fmt.Sprintf("%d", handler.Header), func(ctx context.Context, evt events.Event) {
				eventType := evt.GetType()
				if eventType != fmt.Sprintf("%d", handler.Header) {
					return
				}
				eventValues := evt.GetValues()
				packetEvent := &pb.PacketEvent{
					Id:   evt.GetId(),
					Type: evt.GetType(),
					Packet: &pb.Packet{
						ClientId: eventValues["client_id"].(string),
						Packet:   []byte(eventValues["packet"].(string)),
					},
				}
				handler.Handler(ctx, packetEvent)
			},
			)
		}
	}
}
