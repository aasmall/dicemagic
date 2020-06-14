package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/aasmall/dicemagic/lib/handler"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

//go:generate stringer -type=clientType
type clientType int

const (
	normal clientType = iota
	snooper
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type env struct {
	hub *Hub
}

// Client is an internal representation of a connection to a websocket server
type Client struct {
	hub *Hub

	// The type of client. Snooper will listen to all messages that pass through the hub
	clientType clientType

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// Hub maintains channels for all clients
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	outbound chan []byte

	// Inbound messages to be relayed to evesdroppers
	evesdrop chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

// RTMConnectResponse mocks the response Slack Clients expect from the slack server after a call to .connect
type RTMConnectResponse struct {
	Ok   bool `json:"ok"`
	Self struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"self"`
	Team struct {
		Domain string `json:"domain"`
		ID     string `json:"id"`
		Name   string `json:"name"`
	} `json:"team"`
	URL string `json:"url"`
}

// RTMMessage mocks the json structure of a message from a slack client
type RTMMessage struct {
	Type string `json:"type"`
}

func main() {
	env := &env{hub: newHub()}
	go env.hub.run()

	r := mux.NewRouter()
	r.Handle("/", handler.Handler{Env: env, H: loggerHandler})
	r.Handle("/api/rtm.connect", handler.Handler{Env: env, H: connectHandler})
	r.Handle("/wss/{client-type}", handler.Handler{Env: env, H: wssHandler})
	r.Handle("/api/chat.postMessage", handler.Handler{Env: env, H: postMessageHandler})
	r2 := mux.NewRouter()
	r2.Handle("/", handler.Handler{Env: env, H: loggerHandler})
	r2.Handle("/api/rtm.connect", handler.Handler{Env: env, H: connectHandler})
	r2.Handle("/wss/{client-type}", handler.Handler{Env: env, H: wssHandler})
	r2.Handle("/api/chat.postMessage", handler.Handler{Env: env, H: postMessageHandler})
	// var r2 *mux.Router
	// copier.Copy(r2, r)

	go func() {
		log.Fatal(http.ListenAndServe(":50082", r2))
	}()
	log.Fatal(http.ListenAndServeTLS(":40082", "/etc/mock-tls/tls.crt", "/etc/mock-tls/tls.key", r))
}

func loggerHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusOK)
	log.Printf("SLACK-SERVER-REQUEST: %+v", r)
	return nil
}

func connectHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	response := &RTMConnectResponse{
		Ok: true,
		Self: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "MockTeamID",
			Name: "MockTeamName",
		},
		Team: struct {
			Domain string `json:"domain"`
			ID     string `json:"id"`
			Name   string `json:"name"`
		}{
			Domain: "slackMock",
			ID:     "TeamID",
			Name:   "TeamName",
		},
		URL: "wss://mock-slack-server.default.svc.cluster.local:1082/wss/normal",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	return nil
}
func postMessageHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	decodedValue, err := url.QueryUnescape(string(b))
	if err != nil {
		return err
	}
	e.(*env).hub.evesdrop <- b
	log.Printf("response: %s\n", decodedValue)
	w.WriteHeader(http.StatusOK)
	return nil
}
func wssHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		//log.Printf("questionable origin: %+v", r)
		return true
	}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	client := &Client{hub: e.(*env).hub, conn: conn, send: make(chan []byte, 256)}
	vars := mux.Vars(r)
	if vars["client-type"] == "snooper" {
		client.clientType = snooper
	} else {
		client.clientType = normal
	}
	client.hub.register <- client
	client.send <- sayHello()
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()

	return nil
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		//message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		//if I am a snooping client, send my message directly to normal clients
		//else send the message to snooping clients and process a reply
		if c.clientType == snooper {
			c.hub.outbound <- message
		} else {
			c.hub.evesdrop <- message
			c.hub.outbound <- c.reply(message)
		}
	}
}

type message interface {
	reply() []byte
}

func (m pingMessage) reply() []byte {
	pong := &pongMessage{}
	pong.ID = m.ID
	pong.Timestamp = m.Timestamp
	pong.Type = "pong"
	b, err := json.Marshal(pong)
	if err != nil {
		fmt.Printf("Failed to marshal response: %v.\n", err)
		return nil
	}
	return bytes.TrimSpace(b)
}

type pingMessage struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Timestamp int    `json:"timestamp"`
}
type pongMessage struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Timestamp int    `json:"timestamp"`
}

func (c *Client) reply(m []byte) []byte {
	var obj map[string]interface{}

	err := json.Unmarshal(m, &obj)
	if err != nil {
		fmt.Printf("Failed to unmarshal message: %v.\n", err)
		return nil
	}

	messageType := ""
	if t, ok := obj["type"].(string); ok {
		messageType = t
	}

	var actual message

	switch messageType {
	case "ping":
		actual = &pingMessage{}
	}
	err = json.Unmarshal(m, actual)
	if err != nil {
		fmt.Printf("Failed to unmarshal message: %v.\n", err)
		return nil
	}
	return actual.reply()
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			// n := len(c.send)
			// for i := 0; i < n; i++ {
			// 	w.Write(newline)
			// 	w.Write(<-c.send)
			// }

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func newHub() *Hub {
	return &Hub{
		outbound:   make(chan []byte),
		evesdrop:   make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.outbound:
			for client := range h.clients {
				client.send <- message
			}
		case message := <-h.evesdrop:
			for client := range h.clients {
				if client.clientType == snooper {
					client.send <- message
				}
			}
		}
	}
}
func sayHello() []byte {
	hello := &RTMMessage{Type: "hello"}
	hellob, _ := json.Marshal(hello)
	return hellob
}
