// internal/web/websocket.go
package web

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins in development
    },
}

type WSMessage struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

type WSClient struct {
    conn   *websocket.Conn
    send   chan WSMessage
    server *Server
}

func (s *Server) handleWebSocket(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        logrus.WithError(err).Error("Failed to upgrade websocket")
        return
    }

    client := &WSClient{
        conn:   conn,
        send:   make(chan WSMessage, 256),
        server: s,
    }

    s.wsMu.Lock()
    s.wsClients[client] = true
    s.wsMu.Unlock()

    go client.writePump()
    go client.readPump()
}

func (c *WSClient) writePump() {
    ticker := time.NewTicker(54 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
        c.server.wsMu.Lock()
        delete(c.server.wsClients, c)
        c.server.wsMu.Unlock()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            if err := c.conn.WriteJSON(message); err != nil {
                return
            }

        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}

func (c *WSClient) readPump() {
    defer c.conn.Close()

    c.conn.SetReadLimit(512)
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })

    for {
        _, _, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
    }
}

func (s *Server) broadcast(message WSMessage) {
    s.wsMu.Lock()
    defer s.wsMu.Unlock()

    for client := range s.wsClients {
        select {
        case client.send <- message:
        default:
            close(client.send)
            delete(s.wsClients, client)
        }
    }
}
