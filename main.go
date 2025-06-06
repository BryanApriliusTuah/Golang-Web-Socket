package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var clients = make(map[*websocket.Conn]bool)

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	// Add client to the map
	clients[conn] = true
	defer func() {
		// Remove client from the map on disconnect
		delete(clients, conn)
		conn.Close()
	}()

	fmt.Println("Client connected from : ", conn.RemoteAddr().String())

	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}

		var incoming map[string]any
		err = json.Unmarshal(msg, &incoming)
		if err != nil {
			fmt.Println("Invalid JSON format:", err)
			continue
		}

		broadcastData := map[string]any{
			"from":      conn.RemoteAddr().String(),
			"elevation": incoming["elevation"],
			"latitude":  incoming["latitude"],
			"longitude": incoming["longitude"],
		}

		jsonMsg, err := json.Marshal(broadcastData)
		if err != nil {
			fmt.Println("Error marshaling JSON:", err)
			continue
		}

		for client := range clients {
			if client != conn {
				err := client.WriteMessage(messageType, jsonMsg)
				if err != nil {
					fmt.Println("Error broadcasting to client:", err)
					client.Close()
					delete(clients, client)
				}
			}
		}
	}

}

func main() {
	http.HandleFunc("/ws", handleConnection)

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
