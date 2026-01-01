package main

import (
	"context"

	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
)

func sendClubGiftsComposer(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		composerData, err := data.GetClubGiftsComposerData(ctx, evt.Packet.ClientId)
		if err != nil {
			return
		}
		NetPub.Send(ctx, evt.Packet.ClientId, ClubGiftsComposer(composerData))
	}()
}

func RegisterPacketHandlers() {
	Net.RegisterRedisHandler(487, sendClubGiftsComposer)
}
