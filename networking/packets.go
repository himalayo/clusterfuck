package main

import (
	"encoding/binary"
	"fmt"
)

type preprocessedPacket struct {
	length int
	data []byte
}

func preprocessPacket(data []byte) *preprocessedPacket {
	length := int(binary.BigEndian.Uint32(data[0:4]))
	output := preprocessedPacket{length: length, data: data[4:]}
	return &output
}

func (p *preprocessedPacket) String() string {
	return fmt.Sprintf("%d [% x]", p.length, p.data)
}

type ClientMessage struct {
	header int16
	data []byte
}

func toClientMessage(p *preprocessedPacket) *ClientMessage {
	header := int16(binary.BigEndian.Uint16(p.data[0:2]))
	output := ClientMessage{header: header, data: p.data[2:]}
	return &output
}

func (msg *ClientMessage) String() string {
	return fmt.Sprintf("%d [% x]", msg.header, msg.data)
}

func ParseMessage(unparsedData []byte) *ClientMessage {
	return toClientMessage(preprocessPacket(unparsedData))
}

func ReadShort(data []byte) (int16, []byte) {
	return int16(binary.BigEndian.Uint16(data[0:2])), data[2:]
}

func ReadString(data []byte) (string, []byte) {
	length, remainingData := ReadShort(data)
	return string(remainingData[:length]), remainingData[length:]
}
