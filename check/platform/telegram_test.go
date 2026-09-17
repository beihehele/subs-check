package platform

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func telegramTestDial(mode string) TelegramDialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "blocked" {
			return nil, fmt.Errorf("blocked")
		}
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			server.SetDeadline(time.Now().Add(time.Second))
			request := make([]byte, 42)
			if _, err := io.ReadFull(server, request); err != nil {
				return
			}
			if request[0] != 0xef || request[1] != 10 || binary.LittleEndian.Uint32(request[22:26]) != 0xbe7e8ef1 {
				return
			}
			if mode == "timeout" {
				<-ctx.Done()
				return
			}
			if mode == "http" {
				server.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
				return
			}
			if mode == "oversize" {
				server.Write([]byte{127, 255, 255, 255})
				return
			}
			response := make([]byte, 80)
			binary.LittleEndian.PutUint64(response[8:16], uint64(time.Now().Unix())<<32|1)
			binary.LittleEndian.PutUint32(response[16:20], 60)
			binary.LittleEndian.PutUint32(response[20:24], 0x05162463)
			copy(response[24:40], request[26:42])
			response[56] = 8
			binary.LittleEndian.PutUint32(response[68:72], 0x1cb5c415)
			// 8-byte pq occupies 12 bytes. One fingerprint requires 84 bytes.
			response = append(response, make([]byte, 4)...)
			binary.LittleEndian.PutUint32(response[16:20], 64)
			binary.LittleEndian.PutUint32(response[72:76], 1)
			if mode == "nonce" {
				response[24] ^= 1
			}
			if mode == "vector" {
				response[72] = 2
			}
			if mode == "auth" {
				response[0] = 1
			}
			frame := append([]byte{byte(len(response) / 4)}, response...)
			if mode == "truncated" {
				frame = frame[:len(frame)-3]
			}
			server.Write(frame)
		}()
		return client, nil
	}
}

func TestTelegramRequiresNativeResponse(t *testing.T) {
	for _, mode := range []string{"valid", "nonce", "vector", "auth", "http", "oversize", "truncated", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			r, err := checkTelegramEndpoints(ctx, telegramTestDial(mode), []string{"blocked", "working"})
			if mode == "valid" {
				if err != nil || !r.Reachable {
					t.Fatalf("valid handshake rejected: %v", err)
				}
				return
			}
			if err == nil || r.Reachable {
				t.Fatalf("invalid %s response accepted", mode)
			}
		})
	}
}
