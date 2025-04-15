package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"os"
)

func main() {
	fmt.Println("Client is running")

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	secKey := generateSecKey()
	request := fmt.Sprintf(
		"GET /chat HTTP/1.1\r\n"+
			"Host: localhost:8080\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n\r\n",
		secKey)

	_, err = conn.Write([]byte(request))
	if err != nil {
		fmt.Println("Error writing to server:", err)
		return
	}

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading from server:", err)
		return
	}

	response := string(buffer[:n])
	fmt.Println("Response from server:", response)

	if len(response) > 0 && response[:12] == "HTTP/1.1 101" {
		fmt.Println("WebSocket connection established")
	} else {
		fmt.Println("Failed to establish WebSocket connection")
		return
	}

	// Start listener goroutine
	go func() {
		for {
			msg, err := readFrame(conn)
			if err != nil {
				fmt.Println("Error reading from server:", err)
				return
			}
			fmt.Println("\n[Server]:", msg)
			fmt.Print("Enter message: ")
		}
	}()

	// Input loop for sending
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter message: ")
		text, _ := reader.ReadString('\n')
		text = text[:len(text)-1] // remove newline

		if text == "exit" {
			break
		}

		frame := buildFrame(text)
		_, err := conn.Write(frame)
		if err != nil {
			fmt.Println("Error writing to server:", err)
			return
		}
	}
}

// Generates a random Sec-WebSocket-Key
func generateSecKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// Builds a WebSocket frame from message
func buildFrame(message string) []byte {
	payload := []byte(message)
	frame := []byte{}

	// First byte: FIN=1, Opcode=1 (text frame)
	frame = append(frame, 0x81)

	payloadLength := len(payload)
	if payloadLength <= 125 {
		frame = append(frame, byte(payloadLength)|0x80) // Mask bit set
	} else if payloadLength <= 65535 {
		frame = append(frame, 126|0x80)
		frame = append(frame, byte(payloadLength>>8), byte(payloadLength))
	} else {
		frame = append(frame, 127|0x80)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(payloadLength>>(8*i)))
		}
	}

	// Generate random mask
	maskKey := make([]byte, 4)
	rand.Read(maskKey)
	frame = append(frame, maskKey...)

	// Mask payload
	maskedPayload := make([]byte, payloadLength)
	for i := 0; i < payloadLength; i++ {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}
	frame = append(frame, maskedPayload...)

	return frame
}

// Reads and decodes a WebSocket frame from server
func readFrame(conn net.Conn) (string, error) {
	header := make([]byte, 2)
	_, err := conn.Read(header)
	if err != nil {
		return "", err
	}

	fin := (header[0] & 0x80) != 0
	opcode := header[0] & 0x0F
	mask := (header[1] & 0x80) != 0
	payloadLen := int(header[1] & 0x7F)

	if opcode == 0x8 {
		return "[Server closed the connection]", nil
	}

	if payloadLen == 126 {
		extended := make([]byte, 2)
		_, err := conn.Read(extended)
		if err != nil {
			return "", err
		}
		payloadLen = int(extended[0])<<8 | int(extended[1])
	} else if payloadLen == 127 {
		extended := make([]byte, 8)
		_, err := conn.Read(extended)
		if err != nil {
			return "", err
		}
		// For simplicity, handle up to 32-bit
		payloadLen = int(extended[4])<<24 | int(extended[5])<<16 | int(extended[6])<<8 | int(extended[7])
	}

	if mask {
		// Server should not send masked frames!
		return "", fmt.Errorf("Received masked frame from server, which is invalid!")
	}

	payload := make([]byte, payloadLen)
	_, err = conn.Read(payload)
	if err != nil {
		return "", err
	}

	if !fin {
		return "", fmt.Errorf("Fragmented frames are not yet supported!")
	}

	return string(payload), nil
}
