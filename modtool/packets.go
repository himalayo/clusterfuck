package main

import (
	"encoding/binary"
)

func emptyPacket(header int) []byte {
	return addSize(shortToBytes(header))
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

func boolToByte(b bool) byte {
	if b {
		return byte(1)
	} else {
		return byte(0)
	}
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
