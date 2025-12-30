package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
	plpb "github.com/himalayo/clusterfuck/api/player/proto"
)

func setUserId(ctx context.Context, evt *plpb.LoginEvent) {
	go func() {
		data.rdb.Set(ctx, fmt.Sprintf("user_id:%s", evt.UserData.AuthTicket), evt.UserData.Id, 0).Result()
	}()
}

func loadUserSubscriptions(ctx context.Context, evt *plpb.LoginEvent) {
	go data.loadUserSubscriptionsFromDB(ctx, int(evt.UserData.Id))
}

func sendUserClubComposer(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {

		userIdStr, err := data.rdb.Get(ctx, fmt.Sprintf("user_id:%s", evt.Packet.ClientId)).Result()
		if err != nil {
			return
		}
		user_id, err := strconv.ParseInt(userIdStr, 10, 32)
		if err != nil {
			return
		}
		p := make([]byte, len(evt.Packet.Packet))
		copy(p, evt.Packet.Packet)
		_, p = ReadInt(p)
		_, p = ReadShort(p)
		subscriptionType, _ := ReadString(p)
		subscriptions, err := data.GetUserSubscriptionsByType(ctx, int(user_id), subscriptionType)
		if err != nil {
			return
		}
		log.Printf("Got UserClubEvent from %s: Type: %s", evt.Packet.ClientId, subscriptionType)

		NetPub.Send(ctx, evt.Packet.ClientId, UserClubComposerWithType(ctx, subscriptions, 0, subscriptionType))
	}()
}

func RegisterPacketHandlers() {
	Net.RegisterRedisHandler(3166, sendUserClubComposer)
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(setUserId)
	Player.RegisterLoginHandler(loadUserSubscriptions)
}
