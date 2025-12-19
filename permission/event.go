package main

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
)

func (p RankPerk) Serialize() []byte {
	return serializeValues(p.Key, p.Requirement, p.Value)
}

func (p RankPerk) PushBytes(data []byte) []byte {
	data = appendString(data, p.Key)
	data = appendString(data, p.Requirement)
	data = appendBool(data, p.Value)
	return data
}

func (ps Perks) Serialize() []byte {
	data := make([]byte, 0)
	data = appendInt(data, len(ps))
	for _, perk := range ps {
		data = perk.PushBytes(data)
	}
	return data
}

func UserPermissionsComposer(clubLevel int, permissionLevel int, hasAmbassador bool) []byte {
	return compose(411, clubLevel, permissionLevel, hasAmbassador)
}

func UserPerksComposer(ps Perks) []byte {
	return compose(2586, ps)
}

func handleUserDataRequest(sso string, _ []byte) {
	userData := Player.GetUserData(sso)
	perks, err := db.GetRankPerks(int(userData.Rank))
	if err != nil {
		return
	}
	log.Printf("Sending RankPerks to: %s", sso)
	Net.Send(sso, UserPerksComposer(perks))
}

func sendUserPermissions(ctx context.Context, event *pb.LoginEvent) {
	log.Printf("sendUserPermissions: %s", event.UserData.AuthTicket)
	clubLevel := 0
	if Sub.UserHasSubscription(int(event.UserData.Id), "HABBO_CLUB") {
		clubLevel = 2
	}
	permissionLevel, err := db.GetRankLevel(int(event.UserData.Rank))
	if err != nil {
		return
	}

	hasAmbassador, err := db.GetPermission(int(event.UserData.Rank), "acc_ambassador")
	if err != nil {
		return
	}
	log.Printf("sendUserPermissions: %d", permissionLevel)
	Net.Send(event.UserData.AuthTicket, UserPermissionsComposer(clubLevel, permissionLevel, hasAmbassador == 1))
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(sendUserPermissions)
}

func RegisterIncomingHandlers() {
	Incoming.RegisterHandler(357, handleUserDataRequest)
}
