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
	switch value.(type) {
	case int16:
		return appendShort(packet, int(value.(int16)))
	case int:
		return appendInt(packet, value.(int))
	case string:
		return appendString(packet, value.(string))
	case bool:
		return appendBool(packet, value.(bool))
	case []byte:
		return append(packet, value.([]byte)...)
	case byte:
		return append(packet, value.(byte))
	default:
		return packet
	}
}

func serializeValues(values ...any) []byte {
	binaryData := make([]byte, 0)
	for _, value := range values {
		binaryData = appendValue(binaryData, value)
	}
	return binaryData
}

func compose(data ...any) []byte {
	packet := shortToBytes(data[0].(int))
	packet = append(packet, serializeValues(data[1:]...)...)
	return addSize(packet)
}
