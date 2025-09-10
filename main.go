package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
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

type Level struct {
	Normal int
	Banjir int
}

var (
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex // Protects the clients map
)

func broadcastConnectionCount() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	count := len(clients)
	data := map[string]any{
		"type":             "connection",
		"connection_count": count,
	}

	msg, _ := json.Marshal(data)

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			fmt.Println("Error sending connection count:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func broadcastTimeReady(value any) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	data := map[string]any{
		"type":      "time",
		"timeReady": value,
	}

	msg, _ := json.Marshal(data)

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			fmt.Println("Error sending timeReady:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// broadcast sensor data
func broadcastDataMessage(sender *websocket.Conn, messageType int, broadcastData map[string]any) {
	jsonMsg, err := json.Marshal(broadcastData)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		if client != sender {
			err := client.WriteMessage(messageType, jsonMsg)
			if err != nil {
				fmt.Println("Error broadcasting to client:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	fmt.Println("Client connected from :", conn.RemoteAddr().String())

	// Notify all clients about the new connection count
	broadcastConnectionCount()

	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()

		conn.Close()

		fmt.Println("Client disconnected :", conn.RemoteAddr().String())

		// Notify all clients about the updated connection count
		broadcastConnectionCount()
	}()

	url := "http://host.docker.internal:3000/api/level"
	res, err := http.Get(url)
	if err != nil {
		fmt.Println("Error Fetching Level:", err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error Reading Body Request:", err)
		return
	}

	var level Level
	err = json.Unmarshal(body, &level)
	if err != nil {
		fmt.Println("Failed Decode Body:", err)
	}

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

		// switch by type
		switch incoming["type"] {
		case "time":
			broadcastTimeReady(incoming["timeReady"])

		case "data":
			// evaluate elevation
			var status_elevation string
			elevation := int(incoming["elevation"].(float64))
			if elevation < level.Banjir {
				status_elevation = "Banjir"
			} else if elevation < level.Normal && elevation >= level.Banjir {
				status_elevation = "Siaga"
			} else {
				status_elevation = "Normal"
			}

			// evaluate rainfall
			var status_curah_hujan string
			curah_hujan := int(incoming["curah_hujan"].(float64))
			if curah_hujan > 100 {
				status_curah_hujan = "Hujan deras"
			} else if curah_hujan >= 50 && curah_hujan <= 100 {
				status_curah_hujan = "Hujan lebat"
			} else if curah_hujan >= 20 && curah_hujan < 50 {
				status_curah_hujan = "Hujan sedang"
			} else if curah_hujan >= 5 && curah_hujan < 20 {
				status_curah_hujan = "Gerimis"
			} else {
				status_curah_hujan = "Cerah"
			}

			broadcastData := map[string]any{
				"type":               "data",
				"hardwareId":         incoming["hardwareId"],
				"timestamp":          time.Now().Format(time.RFC1123),
				"elevation":          incoming["elevation"],
				"level_siaga":        incoming["level_siaga"],
				"level_banjir":       incoming["level_banjir"],
				"status_elevation":   status_elevation,
				"curah_hujan":        incoming["curah_hujan"],
				"status_curah_hujan": status_curah_hujan,
				"latitude":           incoming["latitude"],
				"longitude":          incoming["longitude"],
			}

			broadcastDataMessage(conn, messageType, broadcastData)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleConnection)

	fmt.Println("Server started on :8001")
	err := http.ListenAndServe(":8001", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
