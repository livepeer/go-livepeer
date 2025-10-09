package monitor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type mqttBackend struct {
	address   string
	topic     string
	username  string
	password  string
	useTLS    bool
	keepAlive uint16
	qos       byte
	retain    bool
	timeout   time.Duration
}

func init() {
	RegisterBackendFactory("mqtt", newMQTTBackend)
	RegisterBackendFactory("mqtts", newMQTTBackend)
}

func newMQTTBackend(u *url.URL, _ BackendOptions) (EventBackend, error) {
	if u.Host == "" {
		return nil, fmt.Errorf("mqtt sink %q missing host", u.String())
	}

	topic := strings.TrimPrefix(u.Path, "/")
	if topic == "" {
		topic = u.Query().Get("topic")
	}
	if topic == "" {
		return nil, fmt.Errorf("mqtt sink %q missing topic", u.String())
	}

	keepAlive := uint16(60)
	if rawKA := u.Query().Get("keepalive"); rawKA != "" {
		val, err := strconv.Atoi(rawKA)
		if err != nil || val < 0 || val > 0xFFFF {
			return nil, fmt.Errorf("invalid keepalive %q for mqtt sink %s", rawKA, u.String())
		}
		keepAlive = uint16(val)
	}

	qos := byte(0)
	if rawQoS := u.Query().Get("qos"); rawQoS != "" {
		val, err := strconv.Atoi(rawQoS)
		if err != nil || val != 0 {
			return nil, fmt.Errorf("unsupported qos %q for mqtt sink %s (only QoS 0 supported)", rawQoS, u.String())
		}
	}

	retain := false
	if rawRetain := u.Query().Get("retain"); rawRetain != "" {
		retain = rawRetain == "1" || strings.EqualFold(rawRetain, "true")
	}

	timeout := 5 * time.Second
	if rawTimeout := u.Query().Get("timeout"); rawTimeout != "" {
		dur, err := time.ParseDuration(rawTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q for mqtt sink %s: %w", rawTimeout, u.String(), err)
		}
		timeout = dur
	}

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	useTLS := strings.EqualFold(u.Scheme, "mqtts") || u.Query().Get("tls") == "1" || strings.EqualFold(u.Query().Get("tls"), "true")

	return &mqttBackend{
		address:   u.Host,
		topic:     topic,
		username:  username,
		password:  password,
		useTLS:    useTLS,
		keepAlive: keepAlive,
		qos:       qos,
		retain:    retain,
		timeout:   timeout,
	}, nil
}

func (b *mqttBackend) Start(_ context.Context) error {
	return nil
}

func (b *mqttBackend) Publish(ctx context.Context, batch []EventEnvelope) error {
	if len(batch) == 0 {
		return nil
	}

	payload, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	dialer := &net.Dialer{Timeout: b.timeout}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	}

	var conn net.Conn
	if b.useTLS {
		tlsConfig := &tls.Config{ServerName: hostOnly(b.address)}
		conn, err = tls.DialWithDialer(dialer, "tcp", b.address, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", b.address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	clientID := fmt.Sprintf("livepeer-%06d", rand.Intn(999999))

	if err := sendConnectPacket(conn, clientID, b.username, b.password, b.keepAlive); err != nil {
		return err
	}

	if err := awaitConnAck(conn, b.timeout); err != nil {
		return err
	}

	if err := sendPublishPacket(conn, b.topic, payload, b.qos, b.retain); err != nil {
		return err
	}

	sendDisconnectPacket(conn)
	return nil
}

func (b *mqttBackend) Stop(_ context.Context) error {
	return nil
}

func hostOnly(address string) string {
	if idx := strings.LastIndex(address, ":"); idx != -1 {
		return address[:idx]
	}
	return address
}

func sendConnectPacket(conn net.Conn, clientID, username, password string, keepAlive uint16) error {
	var payload []byte
	payload = appendString(payload, clientID)

	flags := byte(0)
	if username != "" {
		flags |= 0x80
		payload = appendString(payload, username)
	}
	if password != "" {
		flags |= 0x40
		payload = appendString(payload, password)
	}

	variableHeader := []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T',
		0x04, // protocol level 4
		flags,
		byte(keepAlive >> 8), byte(keepAlive & 0xFF),
	}

	remainingLength := len(variableHeader) + len(payload)
	packet := []byte{0x10}
	packet = append(packet, encodeRemainingLength(remainingLength)...)
	packet = append(packet, variableHeader...)
	packet = append(packet, payload...)

	_, err := conn.Write(packet)
	return err
}

func awaitConnAck(conn net.Conn, timeout time.Duration) error {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0]>>4 != 0x02 {
		return fmt.Errorf("unexpected MQTT packet 0x%X", header[0])
	}
	remLen := int(header[1])
	buf := make([]byte, remLen)
	if _, err := conn.Read(buf); err != nil {
		return err
	}
	if len(buf) < 2 || buf[1] != 0 {
		return fmt.Errorf("mqtt connection refused: %v", buf)
	}
	return nil
}

func sendPublishPacket(conn net.Conn, topic string, payload []byte, qos byte, retain bool) error {
	fixedHeader := byte(0x30)
	if retain {
		fixedHeader |= 0x01
	}
	fixedHeader |= qos << 1

	var packet []byte
	packet = append(packet, fixedHeader)

	variableHeader := appendString(nil, topic)
	if qos > 0 {
		// Packet Identifier set to 1 for simplicity
		variableHeader = append(variableHeader, 0x00, 0x01)
	}

	remainingLength := len(variableHeader) + len(payload)
	packet = append(packet, encodeRemainingLength(remainingLength)...)
	packet = append(packet, variableHeader...)
	packet = append(packet, payload...)

	_, err := conn.Write(packet)
	return err
}

func sendDisconnectPacket(conn net.Conn) {
	conn.Write([]byte{0xE0, 0x00})
}

func appendString(dst []byte, s string) []byte {
	dst = append(dst, byte(len(s)>>8), byte(len(s)))
	dst = append(dst, s...)
	return dst
}

func encodeRemainingLength(length int) []byte {
	var encoded []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encoded = append(encoded, digit)
		if length == 0 {
			break
		}
	}
	return encoded
}
