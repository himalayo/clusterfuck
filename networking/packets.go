package main

import (
	"encoding/binary"
	"fmt"
)

type preprocessedPacket struct {
	length int
	data   []byte
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
	data   []byte
}

func addSize(packet []byte) []byte {
	return append(integerToBytes(len(packet)), packet...)
}

func integerToBytes(integer int) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, uint32(integer))
	return out
}

func shortToBytes(short int) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(short))
	return out
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

func ReadInt(data []byte) (int, []byte) {
	return int(binary.BigEndian.Uint32(data[0:4])), data[4:]
}

func ReadString(data []byte) (string, []byte) {
	length, remainingData := ReadShort(data)
	return string(remainingData[:length]), remainingData[length:]
}

func appendInt(packet []byte, integer int) []byte {
	return append(packet, integerToBytes(integer)...)
}

func appendShort(packet []byte, short int) []byte {
	return append(packet, shortToBytes(short)...)
}

func appendString(packet []byte, str string) []byte {
	return append(appendShort(packet, len(str)), []byte(str)...)
}

func appendBool(packet []byte, b bool) []byte {
	return append(packet, boolToByte(b))
}

func boolToByte(b bool) byte {
	if b {
		return byte(1)
	} else {
		return byte(0)
	}
}

func appendValue(packet []byte, value any) []byte {
	switch value := value.(type) {
	case int16:
		return appendShort(packet, int(value))
	case int:
		return appendInt(packet, value)
	case string:
		return appendString(packet, value)
	case bool:
		return appendBool(packet, value)
	case []byte:
		return append(packet, value...)
	default:
		return packet
	}
}
func compose(data ...any) []byte {
	packet := shortToBytes(data[0].(int))
	for _, value := range data[1:] {
		packet = appendValue(packet, value)
	}
	return addSize(packet)
}
