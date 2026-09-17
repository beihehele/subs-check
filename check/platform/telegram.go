package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type TelegramResult struct{ Reachable bool }
type TelegramDialer func(context.Context, string, string) (net.Conn, error)

// CheckTelegram probes the first unauthenticated MTProto exchange over the
// node's own TCP dialer. TG means a DC answered req_pq_multi with matching resPQ;
// it does not claim account login, media downloads, or voice-call availability.
// Protocol: https://core.telegram.org/mtproto/auth_key
// Framing: https://core.telegram.org/mtproto/mtproto-transports#abridged
func CheckTelegram(ctx context.Context, dial TelegramDialer) (*TelegramResult, error) {
	return checkTelegramEndpoints(ctx, dial, []string{"149.154.167.51:443", "149.154.175.50:443"})
}

func checkTelegramEndpoints(ctx context.Context, dial TelegramDialer, endpoints []string) (*TelegramResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(endpoints))
	for _, endpoint := range endpoints {
		go func() { results <- probeTelegram(ctx, dial, endpoint) }()
	}
	var last error
	for range endpoints {
		select {
		case <-ctx.Done():
			return &TelegramResult{}, ctx.Err()
		case err := <-results:
			if err == nil {
				return &TelegramResult{Reachable: true}, nil
			}
			last = err
		}
	}
	return &TelegramResult{}, last
}

func probeTelegram(ctx context.Context, dial TelegramDialer, endpoint string) error {
	conn, err := dial(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline := time.Now().Add(10 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	// Unencrypted envelope: auth_key_id(0), msg_id, body length, req_pq_multi.
	packet := make([]byte, 42)
	packet[0], packet[1] = 0xef, 10 // abridged marker + 40-byte payload / 4
	now := time.Now()
	id := uint64(now.Unix())<<32 | uint64(now.Nanosecond())*(1<<32)/1e9
	binary.LittleEndian.PutUint64(packet[10:18], id&^3)
	binary.LittleEndian.PutUint32(packet[18:22], 20)
	binary.LittleEndian.PutUint32(packet[22:26], 0xbe7e8ef1)
	copy(packet[26:], nonce[:])
	if _, err := io.Copy(conn, bytes.NewReader(packet)); err != nil {
		return err
	}
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:1]); err != nil {
		return err
	}
	words := int(header[0])
	if words == 127 {
		if _, err := io.ReadFull(conn, header[1:]); err != nil {
			return err
		}
		words = int(header[1]) | int(header[2])<<8 | int(header[3])<<16
	}
	if words < 16 || words > 1024 {
		return fmt.Errorf("invalid Telegram frame length: %d", words*4)
	}
	response := make([]byte, words*4)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}
	if binary.LittleEndian.Uint64(response[:8]) != 0 || int(binary.LittleEndian.Uint32(response[16:20])) != len(response)-20 || binary.LittleEndian.Uint32(response[20:24]) != 0x05162463 || !bytes.Equal(response[24:40], nonce[:]) {
		return fmt.Errorf("invalid Telegram resPQ or nonce")
	}
	// Verify TL string padding and the public-key-fingerprint vector, rather
	// than accepting an echoed prefix or a generic TCP/HTTP response.
	pqLen := int(response[56])
	if pqLen < 1 || pqLen > 8 {
		return fmt.Errorf("invalid Telegram pq")
	}
	offset := 56 + (1+pqLen+3)/4*4
	if offset+8 > len(response) || binary.LittleEndian.Uint32(response[offset:]) != 0x1cb5c415 {
		return fmt.Errorf("invalid Telegram fingerprints")
	}
	count := int(binary.LittleEndian.Uint32(response[offset+4:]))
	if count < 1 || count > 64 || offset+8+count*8 != len(response) {
		return fmt.Errorf("invalid Telegram fingerprint count")
	}
	return nil
}
