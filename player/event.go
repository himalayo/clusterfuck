package main

import (
	"log"

	achievements "github.com/himalayo/clusterfuck/api/achievements"
	configuration "github.com/himalayo/clusterfuck/api/configuration"
	modtool "github.com/himalayo/clusterfuck/api/modtool"
	networking "github.com/himalayo/clusterfuck/api/networking"
	permission "github.com/himalayo/clusterfuck/api/permission"
	pb "github.com/himalayo/clusterfuck/api/player/proto"
	subscription "github.com/himalayo/clusterfuck/api/subscription"
)

type LoginEvent struct {
	Data          *Database
	UserData      *UserData
	Network       *networking.NetworkingClient
	Subscription  *subscription.SubscriptionClient
	Permission    *permission.PermissionsClient
	Modtool       *modtool.ModtoolClient
	Configuration *configuration.ConfigurationClient
	Achievements  *achievements.AchievementsClient
}

func (l *LoginEvent) Send(data []byte) {
	if data != nil {
		l.Network.Send(l.UserData.AuthTicket, data)
	}
}

type EventListener struct {
	ticket        chan *pb.Ticket
	data          *Database
	netw          *networking.NetworkingClient
	sub           *subscription.SubscriptionClient
	perm          *permission.PermissionsClient
	mod           *modtool.ModtoolClient
	cfg           *configuration.ConfigurationClient
	ach           *achievements.AchievementsClient
	login         chan bool
	loginHandlers []func(*LoginEvent) []byte
}

func NewEventListener(data *Database, netw *networking.NetworkingClient, sub *subscription.SubscriptionClient, perm *permission.PermissionsClient, mod *modtool.ModtoolClient, cfg *configuration.ConfigurationClient, ach *achievements.AchievementsClient) *EventListener {
	return &EventListener{ticket: make(chan *pb.Ticket), data: data, netw: netw, login: make(chan bool), sub: sub, perm: perm, mod: mod, cfg: cfg, ach: ach}
}

func (e *EventListener) newLoginEvent(userData *UserData) *LoginEvent {
	return &LoginEvent{Data: e.data, UserData: userData, Network: e.netw, Subscription: e.sub, Permission: e.perm, Modtool: e.mod, Configuration: e.cfg, Achievements: e.ach}
}

func (e *EventListener) handleLoginEvent(ticket string) {
	log.Printf("EventListener.handleLoginEvent: %s", ticket)
	e.data.AuthTicketEvent(ticket)
}

func (e *EventListener) resultLoginEvent(playerData *UserData) {
	e.login <- playerData != nil
	event := e.newLoginEvent(playerData)
	event.Send(e.loginHandlers[0](event))
	for _, handler := range e.loginHandlers[1:] {
		go func(event *LoginEvent, handler func(*LoginEvent) []byte) {
			event.Send(handler(event))
		}(event, handler)
	}
}

func (e *EventListener) RegisterLoginHandler(handler func(*LoginEvent) []byte) {
	e.loginHandlers = append(e.loginHandlers, handler)
}

func (e *EventListener) Login(sso *pb.Ticket) bool {
	log.Printf("EventListener.Login: %s", sso)
	e.ticket <- sso
	out := <-e.login
	return out
}

func handleUserDataRequest(sso string, _ []byte) {
	log.Printf("%s: HandleUserDataRequest called!", sso)
}

func RegisterIncomingHandlers() {
	Incoming.RegisterHandler(357, handleUserDataRequest)
}

func (e *EventListener) Listen() {
	RegisterLoginHandlers(e)
	RegisterIncomingHandlers()
	for {
		select {
		case ticket := <-e.ticket:
			e.handleLoginEvent(ticket.Sso)
		case playerData := <-e.data.Auth:
			e.resultLoginEvent(playerData)
		}
	}
}
