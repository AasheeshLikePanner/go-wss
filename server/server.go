package main

import (
	"crypto/sha1"
    "encoding/base64"
    "fmt"
    "io"
    "net"
    "strings"
)

func main(){
	fmt.Println("Server is running")

	listener, err := net.Listen("tcp", ":8080");
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept();
		if err != nil {
			fmt.Println("Error accepting connectionL", err);
			continue
		}
		go handleConnection(conn);
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024);
	n, err := conn.Read(buffer);
	if err != nil {
		fmt.Println("Error reading from client:", err);
		return
	}
	request := string(buffer[:n]);
	lines := strings.Split(request, "\r\n");

	if(len(lines) < 1){
		fmt.Println("Error: Invalid request")
		return;
	}

	var secWebSocketKey string;

	for _ ,line := range lines {
		if (strings.HasPrefix(line, "Sec-WebSocket-Key:")) {
			secWebSocketKey = strings.TrimSpace(strings.Split(line, ":")[1]);
			break;
		}
	}
	if (secWebSocketKey == "") {
		fmt.Println("Error: Sec-WebSocket-Key not found in request")
		return;
	}
	acceptedKey := generateAcceptKey(secWebSocketKey);

	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n\r\n",
		acceptedKey);	

	conn.Write([]byte(response));
	fmt.Println("Handshake complete with client.")

	for {
		header := make([]byte, 2);
		_, err = io.ReadFull(conn,header);
		if err != nil {
			fmt.Println("Error reading from client:", err);
			return;
		}
		fin := header[0] & 0x80 != 0;
		opcode := header[0] & 0x0F;
		masked := header[1] & 0x80 != 0;
		payloadLength := header[1] & 0x7F;
		if !masked {
			fmt.Println("Error: Unmasked frame received")
			return;
		}

		maskey := make([]byte, 4);
		_, err = io.ReadFull(conn, maskey);
		if err != nil {
			fmt.Println("Error reading from client:", err);
			return;
		}

		payload := make([]byte, payloadLength);
		_, err = io.ReadFull(conn, payload);
		if err != nil {
			fmt.Println("Error reading from client:", err);
			return;
		}
		for i := 0; i < int(payloadLength); i++ {
			payload[i] = payload[i] ^ maskey[i % 4];
		}
		conn.Write(payload);
		recivedMessage := string(payload);
		fmt.Println("Received message from client:", recivedMessage);
		
		if fin && opcode == 1	{
			response := buildFrame([]byte("ECHO: " + recivedMessage));
			conn.Write(response);
		}

	}
}

func generateAcceptKey(secWebSocketKey string) string{
	magicString := "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";
	sha := sha1.New();
	sha.Write([]byte(secWebSocketKey + magicString));
	return base64.StdEncoding.EncodeToString(sha.Sum(nil));
}

func buildFrame(payload []byte) []byte {
	frame := make([]byte, 2 + len(payload));
	length := len(payload);
	frame[0] = 0x81; // FIN + opcode
	frame[1] = byte(length); // payload length
	copy(frame[2:], payload);
	return frame;
}