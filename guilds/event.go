package main

import (
	"context"
	"log"
	"sync"

	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
)

func sendGuildPartsComposer(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		var wg sync.WaitGroup
		err_ch := make(chan error, 5)
		bases := make([]GuildPart, 0)
		symbols := make([]GuildPart, 0)
		base_colors := make([]GuildPart, 0)
		symbol_colors := make([]GuildPart, 0)
		background_colors := make([]GuildPart, 0)
		wg.Go(func() {
			var err error
			bases, err = data.LoadGuildPartsByType(ctx, "base")
			err_ch <- err
		})
		wg.Go(func() {
			var err error
			symbols, err = data.LoadGuildPartsByType(ctx, "symbol")
			err_ch <- err
		})
		wg.Go(func() {
			var err error
			base_colors, err = data.LoadGuildPartsByType(ctx, "base_color")
			err_ch <- err
		})
		wg.Go(func() {
			var err error
			symbol_colors, err = data.LoadGuildPartsByType(ctx, "symbol_color")
			err_ch <- err
		})
		wg.Go(func() {
			var err error
			background_colors, err = data.LoadGuildPartsByType(ctx, "background_color")
			err_ch <- err
		})
		wg.Wait()
		close(err_ch)
		for err := range err_ch {
			if err != nil {
				log.Printf("sendGuildPartsComposer(): Got error while loading parts: %v", err)
			}
		}
		packet := GuildPartsComposer([][]GuildPart{bases, symbols, base_colors, symbol_colors, background_colors})
		NetPub.Send(ctx, evt.Packet.ClientId, packet)
	}()
}

func RegisterPacketHandlers() {
	Net.RegisterRedisHandler(813, sendGuildPartsComposer)
}
