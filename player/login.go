package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

func RegisterLoginHandlers(e *EventListener) {
	e.RegisterLoginHandler(sendLoginOK)
	e.RegisterLoginHandler(loadHCData)
	e.RegisterLoginHandler(setUserId)
	e.RegisterLoginHandler(sendUserEffects)
	e.RegisterLoginHandler(sendUserNoobStatus)
	e.RegisterLoginHandler(sendAvailabilityStatus)
	e.RegisterLoginHandler(sendEnableNotifications)
	e.RegisterLoginHandler(sendAchievementScore)
	e.RegisterLoginHandler(sendMysteryBox)
	e.RegisterLoginHandler(sendBuildersClubExpired)
	e.RegisterLoginHandler(sendFavoriteRooms)
}

func loadHCData(ctx context.Context, event *LoginEvent) {
	go func() {
		Data.cacheUserHCData(ctx, event.UserData.AuthTicket)
	}()
}

func setUserId(ctx context.Context, event *LoginEvent) {
	go func() {
		Data.cache.Set(ctx, fmt.Sprintf("user_id:%s", event.UserData.AuthTicket), event.UserData.Id, 0).Result()
	}()
}

func sendLoginOK(ctx context.Context, event *LoginEvent) {
	log.Printf("sendLoginOK: %s", event.UserData.AuthTicket)
	NetPub.Send(ctx, event.UserData.AuthTicket, LoginOKComposer())
}

func sendUserNoobStatus(ctx context.Context, event *LoginEvent) {
	log.Printf("sendUserNoobStatus: %s", event.UserData.AuthTicket)
	NetPub.Send(ctx, event.UserData.AuthTicket, UserNoobStatusComposer(1))
}

func sendUserEffects(ctx context.Context, event *LoginEvent) {
	log.Printf("sendUserEffects: %s", event.UserData.AuthTicket)
	var packet = shortToBytes(340)
	Data.UserEffectsEvent(event.UserData.Id)
	effects := <-Data.Effects
	if effects == nil {
		log.Printf("sendUserEffects: sending null")
		addSize(appendInt(packet, 0))
	}
	packet = appendInt(packet, len(effects))
	for _, userEffect := range effects {
		packet = serializeUserEffect(packet, &userEffect)
	}
	packet = addSize(packet)
	NetPub.Send(ctx, event.UserData.AuthTicket, packet)
}

func sendAvailabilityStatus(ctx context.Context, event *LoginEvent) {
	NetPub.Send(ctx, event.UserData.AuthTicket, AvailabilityStatusComposer(true, false, true))
}

func sendEnableNotifications(ctx context.Context, event *LoginEvent) {
	NetPub.Send(ctx, event.UserData.AuthTicket, EnableNotificationsComposer(true))
}

func sendAchievementScore(ctx context.Context, event *LoginEvent) {
	NetPub.Send(ctx, event.UserData.AuthTicket, AchievementScoreComposer(Data.getAchievementScore(event.UserData.Id)))
}

func sendMysteryBox(ctx context.Context, event *LoginEvent) {
	NetPub.Send(ctx, event.UserData.AuthTicket, MysteryBoxComposer())
}

func sendBuildersClubExpired(ctx context.Context, event *LoginEvent) {
	NetPub.Send(ctx, event.UserData.AuthTicket, BuildersClubExpiredComposer())
}

func sendFavoriteRooms(ctx context.Context, event *LoginEvent) {
	maxFavoriteRooms, err := Cfg.GetInt("hotel.rooms.max.favorite")
	if err != nil {
		log.Printf("sendFavoriteRoomsComposer: %v", err)
		return
	}
	favoriteRooms := Data.getFavoriteRooms(event.UserData.Id)
	if favoriteRooms == nil {
		favoriteRooms = []int{}
	}
	log.Printf("sendFavoriteRoomsComposer: %d %v", maxFavoriteRooms, favoriteRooms)

	NetPub.Send(ctx, event.UserData.AuthTicket, FavoriteRoomsCountComposer(maxFavoriteRooms, favoriteRooms))
}

func serializeUserEffect(packet []byte, effect *UserEffect) []byte {
	var out = appendInt(packet, effect.Effect)
	out = appendInt(out, 0)
	if effect.Duration > 0 {
		out = appendInt(out, effect.Duration)
		if effect.ActivationTimestamp >= 0 {
			out = appendInt(out, effect.Total-1)
		} else {
			out = appendInt(out, effect.Total)
		}
	} else {
		out = appendInt(out, math.MaxUint32)
		out = appendInt(out, 0)
	}

	if !(effect.ActivationTimestamp >= 0) && effect.Duration > 0 {
		out = appendInt(out, 0)
	} else {
		if effect.Duration > 0 {
			out = appendInt(out, int(time.Now().Unix())-effect.ActivationTimestamp+effect.Duration)
		} else {
			out = appendInt(out, 0)
		}
	}
	out = appendBool(out, true)
	return out
}
