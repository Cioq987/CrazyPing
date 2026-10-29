package protocol

import (
	"encoding/binary"
	"errors"
	"net"
	"time"
)

const (
	MagicByte1 byte = 0x43 // 'C'
	MagicByte2 byte = 0x50 // 'P'

	TypeData byte = 0x01
	TypePing byte = 0x02
	TypePong byte = 0x03
	TypeAdminReq  byte = 0x10
	TypeAdminResp byte = 0x11
	TypeEnrollReq  byte = 0x20
	TypeEnrollResp byte = 0x21

	HeaderSize = 2 + 1 + 4 + 8 + 4 + 2 + 2 // 23 bytes
)

var (
	ErrPacketTooShort = errors.New("packet too short")
	ErrInvalidMagic   = errors.New("invalid magic header")
)

// Packet represents a CrazyPing encapsulation envelope.
type Packet struct {
	Type       byte
	Seq        uint32
	Timestamp  int64 // UnixNano
	TargetIP   net.IP
	TargetPort uint16
	Payload    []byte
}

// Encode serializes the Packet into a byte slice.
func (p *Packet) Encode() []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))
	buf[0] = MagicByte1
	buf[1] = MagicByte2
	buf[2] = p.Type
	binary.BigEndian.PutUint32(buf[3:7], p.Seq)
	binary.BigEndian.PutUint64(buf[7:15], uint64(p.Timestamp))

	ipv4 := p.TargetIP.To4()
	if ipv4 == nil {
		ipv4 = net.IPv4zero.To4()
	}
	copy(buf[15:19], ipv4)
	binary.BigEndian.PutUint16(buf[19:21], p.TargetPort)
	binary.BigEndian.PutUint16(buf[21:23], uint16(len(p.Payload)))
	copy(buf[23:], p.Payload)

	return buf
}

// Decode deserializes bytes into a Packet.
func Decode(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, ErrPacketTooShort
	}
	if data[0] != MagicByte1 || data[1] != MagicByte2 {
		return nil, ErrInvalidMagic
	}

	p := &Packet{
		Type:       data[2],
		Seq:        binary.BigEndian.Uint32(data[3:7]),
		Timestamp:  int64(binary.BigEndian.Uint64(data[7:15])),
		TargetIP:   net.IPv4(data[15], data[16], data[17], data[18]),
		TargetPort: binary.BigEndian.Uint16(data[19:21]),
	}

	payloadLen := int(binary.BigEndian.Uint16(data[21:23]))
	if len(data) >= HeaderSize+payloadLen {
		p.Payload = data[HeaderSize : HeaderSize+payloadLen]
	} else {
		p.Payload = data[HeaderSize:]
	}

	return p, nil
}

// NewPing creates a ping measurement packet.
func NewPing(seq uint32) *Packet {
	return &Packet{
		Type:      TypePing,
		Seq:       seq,
		Timestamp: time.Now().UnixNano(),
	}
}

// NewData creates an encapsulated game data packet.
func NewData(seq uint32, targetIP net.IP, targetPort uint16, payload []byte) *Packet {
	return &Packet{
		Type:       TypeData,
		Seq:        seq,
		Timestamp:  time.Now().UnixNano(),
		TargetIP:   targetIP,
		TargetPort: targetPort,
		Payload:    payload,
	}
}
