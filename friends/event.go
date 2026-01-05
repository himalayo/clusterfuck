package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"slices"
	"sync"

	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
	plpb "github.com/himalayo/clusterfuck/api/player/proto"
)

func cachePermissionData(ctx context.Context, evt *plpb.LoginEvent) {
	go func() {
		log.Printf("Caching permission data for: %s", evt.UserData.AuthTicket)
		rank := Perm.GetRank(int(evt.GetUserData().GetRank()))
		if rank == nil {
			return
		}
		pipe := data.rdb.Pipeline()
		for permissionKey, permission := range rank.Permissions {
			key := fmt.Sprintf("user_permission:%s:%s", evt.UserData.AuthTicket, permissionKey)
			log.Printf("Setting %v", key)

			pipe.Set(ctx, key, int(permission.GetSetting()), 0)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Got error executing cache permission data pipeline: %v", err)
			return
		}

		for _, c := range cmds {
			if err := c.Err(); err != nil {
				log.Printf("Got error caching permission data: %v", err)
			}
		}
	}()
}

func setUserRank(ctx context.Context, evt *plpb.LoginEvent) {
	go func() {
		data.rdb.Set(ctx, fmt.Sprintf("user_rank:%s", evt.UserData.AuthTicket), evt.UserData.Rank, 0).Result()
	}()
}
func setUserId(ctx context.Context, evt *plpb.LoginEvent) {
	go func() {
		data.rdb.Set(ctx, fmt.Sprintf("user_id:%s", evt.UserData.AuthTicket), evt.UserData.Id, 0).Result()
	}()
}

func sendMessengerInitComposer(ctx context.Context, evt *netpb.PacketEvent) {
	var wg sync.WaitGroup
	var infiniteFriends bool
	var categories []*MessengerCategory = nil
	wg.Go(
		func() {
			infiniteFriendsPointer, err := data.GetPermission(ctx, evt.Packet.ClientId, "acc_infinite_friends")
			if err != nil {
				log.Printf("sendMessengerInitComposer(): Got error while fetching infinite friends permission: %v", err)
			}
			if infiniteFriendsPointer == nil {
				infiniteFriends = false
				return
			}
			infiniteFriends = *infiniteFriendsPointer
		},
	)
	wg.Go(
		func() {
			cats, err := data.GetMessengerCategories(ctx, evt.Packet.ClientId)
			if err != nil {
				log.Printf("sendMessengerInitComposer(): Got error while fetching messenger categories: %v", err)
				return
			}
			categories = cats
		},
	)
	wg.Wait()
	packet := MessengerInitComposer(&MessengerInitComposerData{
		InfiniteFriends:     infiniteFriends,
		MessengerCategories: categories,
	})
	log.Printf("sendMessengerInitComposer(): Sending to %s: [% x]", evt.Packet.ClientId, packet)
	NetPub.Send(ctx, evt.Packet.ClientId, packet)
}

func sendFriendsComposer(ctx context.Context, evt *netpb.PacketEvent) {
	friends, err := data.GetFriendsForUser(ctx, evt.Packet.ClientId)
	if err != nil {
		log.Printf("sendFriendsComposers(): Got error while fetching users: %v", err)
		return
	}
	totalPages := int(math.Ceil(float64(len(friends)) / 750.0))
	pageIndex := 0
	for page := range slices.Chunk(friends, 750) {
		packet := FriendsComposer(totalPages, pageIndex, page)
		log.Printf("sendFriendsComposers(): Sending: %v", packet)
		NetPub.Send(ctx, evt.Packet.ClientId, packet)
		pageIndex++
	}
	if pageIndex == 0 {
		packet := FriendsComposer(totalPages, pageIndex, friends)
		log.Printf("sendFriendsComposers(): Sending: %v", packet)
		NetPub.Send(ctx, evt.Packet.ClientId, packet)
	}
}

func sendInitialComposers(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		sendMessengerInitComposer(ctx, evt)
		sendFriendsComposer(ctx, evt)
	}()
}

func sendLoadFriendRequestsComposer(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		requests, err := data.GetFriendRequestsForUser(ctx, evt.Packet.ClientId)
		if err != nil {
			log.Printf("sendFriendRequests(): Got error: %v", err)
			return
		}

		NetPub.Send(ctx, evt.Packet.ClientId, LoadFriendRequestsComposer(requests))
	}()
}

func RegisterPacketHandlers() {
	Net.RegisterRedisHandler(2781, sendInitialComposers)
	Net.RegisterRedisHandler(2448, sendLoadFriendRequestsComposer)
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(setUserId)
	Player.RegisterLoginHandler(setUserRank)
	Player.RegisterLoginHandler(cachePermissionData)
}
