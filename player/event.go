package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	events "github.com/himalayo/clusterfuck/api/events"
	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
	pb "github.com/himalayo/clusterfuck/api/player/proto"
	"github.com/redis/go-redis/v9"
)

type LoginEvent struct {
	Id       string
	Type     string
	Values   map[string]interface{}
	UserData *UserData
}

func ToLoginEvent(evt events.Event) *LoginEvent {
	return &LoginEvent{
		Id:       evt.GetId(),
		Type:     evt.GetType(),
		Values:   evt.GetValues(),
		UserData: NewUserDataFromMap(evt.GetValues()),
	}
}

const (
	LoginEventType = "LOGIN"
)

func (l *LoginEvent) Send(data []byte) {
	if data != nil {
		NetPub.Send(context.Background(), l.UserData.AuthTicket, data)
	}
}

func (l *LoginEvent) GetId() string {
	return l.Id
}

func (l *LoginEvent) GetType() string {
	return l.Type
}

func (l *LoginEvent) GetValues() map[string]interface{} {
	return l.Values
}

type EventListener struct {
	ticket    chan *pb.Ticket
	redis_pub *events.RedisPublisher
	bus       *events.LocalBus
	login     chan bool
}

func NewEventListener(evt_cfg *redis.Options) *EventListener {
	redis_pub := events.NewRedisPublisher(evt_cfg, "player-events")
	bus := events.NewLocalBus()
	return &EventListener{
		ticket:    make(chan *pb.Ticket),
		login:     make(chan bool),
		redis_pub: redis_pub, bus: bus,
	}
}

func (u *UserData) ToMap() map[string]interface{} {
	values := make(map[string]interface{})
	values["user_id"] = u.Id
	values["username"] = u.Username
	values["auth_ticket"] = u.AuthTicket
	values["look"] = u.Look
	values["motto"] = u.Motto
	values["home_room"] = u.HomeRoom
	values["rank"] = u.Rank
	return values
}

func NewUserDataFromMap(values map[string]interface{}) *UserData {
	return &UserData{
		Id:         values["user_id"].(int),
		Username:   values["username"].(string),
		AuthTicket: values["auth_ticket"].(string),
		Look:       values["look"].(string),
		Motto:      values["motto"].(string),
		HomeRoom:   values["home_room"].(int),
		Rank:       values["rank"].(int),
	}
}

func (e *EventListener) newLoginEvent(userData *UserData) *LoginEvent {
	id_uuid, err := uuid.NewRandom()
	var id string
	if err != nil {
		id = fmt.Sprintf("%v", rand.Float64())
	} else {
		id = id_uuid.String()
	}
	eventType := LoginEventType
	values := userData.ToMap()
	values["id"] = id
	values["type"] = eventType
	return &LoginEvent{
		Id: id, Type: eventType, Values: values,
		UserData: userData,
	}
}

func (e *EventListener) handleLoginEvent(ticket string) {
	log.Printf("EventListener.handleLoginEvent: %s", ticket)
	Data.AuthTicketEvent(ticket)
}

func (e *EventListener) resultLoginEvent(ctx context.Context, playerData *UserData) {
	e.login <- playerData != nil
	event := e.newLoginEvent(playerData)
	go e.redis_pub.Publish(ctx, event)
	go e.bus.Publish(ctx, event)
}

func (e *EventListener) RegisterLoginHandler(handler func(context.Context, *LoginEvent)) {
	e.bus.Subscribe(LoginEventType, func(ctx context.Context, evt events.Event) {
		handler(ctx, ToLoginEvent(evt))
	})
}

func (e *EventListener) Login(sso *pb.Ticket) bool {
	log.Printf("EventListener.Login: %s", sso)
	e.ticket <- sso
	out := <-e.login
	return out
}

func handleUserDataRequest(sso string, _ []byte) {
	info := Data.loadUserInfoComposerData(sso)
	log.Printf("%s: Sending UserDataComposer: %s", sso, info.String())
	Net.Send(sso, UserDataComposer(info))
}

func RegisterIncomingHandlers() {
	Incoming.RegisterHandler(357, handleUserDataRequest)
}

func handleUserCreditsRequest(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		credits, err := Data.GetUserCredits(ctx, evt.Packet.ClientId)
		if err != nil {
			return
		}
		Net.Send(evt.Packet.ClientId, UserCreditsComposer(credits))
	}()
	go func() {
		var wg sync.WaitGroup
		var types []int
		var amounts map[int]int
		wg.Go(
			func() {
				types_str, err := Cfg.GetStringOrDefault("seasonal.types", "0;1;2;3;4;5;101;102;103;104;105")
				if err != nil {
					return
				}
				typesStr := strings.Split(types_str, ";")
				for _, typeStr := range typesStr {
					currencyType, err := strconv.ParseInt(typeStr, 10, 32)
					if err != nil {
						continue
					}
					types = append(types, int(currencyType))
				}
			},
		)
		wg.Go(
			func() {
				amounts, _ = Data.GetUserCurrencies(ctx, evt.Packet.ClientId)
			},
		)
		wg.Wait()
		currencies := make([]UserCurrency, len(types))
		for i, currencyType := range types {
			amount, ok := amounts[currencyType]
			if !ok {
				amount = 0
			}
			currencies[i] = UserCurrency{
				Type:   currencyType,
				Amount: amount,
			}
		}
		Net.Send(evt.Packet.ClientId, UserCurrencyComposer(currencies))
	}()
}

func RegisterRedisHandlers() {
	Net.RegisterRedisHandler(273, handleUserCreditsRequest)
}

func (e *EventListener) Listen() {
	RegisterLoginHandlers(e)
	RegisterIncomingHandlers()
	RegisterRedisHandlers()
	for {
		select {
		case ticket := <-e.ticket:
			e.handleLoginEvent(ticket.Sso)
		case playerData := <-Data.Auth:
			e.resultLoginEvent(context.Background(), playerData)
		}
	}
}
