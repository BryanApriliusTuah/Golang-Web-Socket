package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

	clients[conn] = true
	defer func() {
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

		var status_elevation string
		elevation := int(incoming["elevation"].(float64))
		if elevation > 100 {
			status_elevation = "Normal"
		} else if elevation < 80 {
			status_elevation = "Banjir"
		} else {
			status_elevation = "Siaga"
		}

		var status_curah_hujan string
		curah_hujan := int(incoming["curah_hujan"].(float64))
		if curah_hujan >= 50 {
			status_curah_hujan = "Hujan deras"
		} else if curah_hujan >= 20 && curah_hujan < 50 {
			status_curah_hujan = "Hujan sedang"
		} else if curah_hujan > 0 && curah_hujan < 20 {
			status_curah_hujan = "Hujan ringan"
		} else {
			status_curah_hujan = "Tidak ada hujan"
		}

		broadcastData := map[string]any{
			"from":               conn.RemoteAddr().String(),
			"timestamp":          time.Now().Format(time.RFC1123),
			"elevation":          incoming["elevation"],
			"status_elevation":   status_elevation,
			"curah_hujan":        incoming["curah_hujan"],
			"status_curah_hujan": status_curah_hujan,
			"latitude":           incoming["latitude"],
			"longitude":          incoming["longitude"],
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
