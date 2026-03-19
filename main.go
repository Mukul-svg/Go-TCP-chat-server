package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

type Message struct {
	Type    string `json:"type"`
	Sender  string `json:"sender,omitempty"`
	Target  string `json:"target,omitempty"`
	Room    string `json:"room,omitempty"`
	Content string `json:"content"`
}

type Connection struct {
	nickname string
	room     string
	conn     net.Conn
}

type ClientRegistry struct {
	connections map[string]Connection
	mu          sync.RWMutex
}

func sendJSON(conn net.Conn, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}

func executeCommand(input string, addr string, connReg *ClientRegistry, currentNick string) (string, bool) {
	parts := strings.SplitN(input, " ", 3)
	if len(parts) == 0 {
		return currentNick, false
	}
	command := parts[0]

	connReg.mu.RLock()
	senderConn := connReg.connections[addr].conn
	connReg.mu.RUnlock()

	if command == "nick" {
		if len(parts) < 2 {
			sendJSON(senderConn, Message{Type: "error", Content: "Usage: /nick <new_nickname>"})
			return currentNick, false
		}
		newNick := parts[1]

		connReg.mu.Lock()
		sender := connReg.connections[addr]
		sender.nickname = newNick
		connReg.connections[addr] = sender
		connReg.mu.Unlock()

		sendJSON(senderConn, Message{Type: "system", Content: fmt.Sprintf("Nickname updated to %s", newNick)})
		return newNick, false
	}

	if command == "list" {
		sendList(senderConn, connReg)
		return currentNick, false
	}

	if command == "join" {
		if len(parts) < 2 {
			sendJSON(senderConn, Message{Type: "error", Content: "Usage: /join <room>"})
			return currentNick, false
		}
		room := parts[1]

		connReg.mu.Lock()
		sender := connReg.connections[addr]
		oldRoom := sender.room
		sender.room = room
		connReg.connections[addr] = sender
		connReg.mu.Unlock()

		if oldRoom != "" && oldRoom != room {
			broadcast(addr, Message{Type: "system", Room: oldRoom, Content: fmt.Sprintf("%s has left the room.", currentNick)}, connReg)
		}
		if oldRoom != room {
			broadcast(addr, Message{Type: "system", Room: room, Content: fmt.Sprintf("%s has joined the room!", currentNick)}, connReg)
			sendJSON(senderConn, Message{Type: "system", Content: fmt.Sprintf("You joined room: %s", room)})
		}
		return currentNick, false
	}

	if command == "leave" {
		connReg.mu.Lock()
		sender := connReg.connections[addr]
		oldRoom := sender.room
		sender.room = ""
		connReg.connections[addr] = sender
		connReg.mu.Unlock()

		if oldRoom != "" {
			broadcast(addr, Message{Type: "system", Room: oldRoom, Content: fmt.Sprintf("%s has left the room.", currentNick)}, connReg)
			sendJSON(senderConn, Message{Type: "system", Content: fmt.Sprintf("You left room: %s", oldRoom)})
		}
		return currentNick, false
	}

	if command == "msg" {
		if len(parts) < 3 {
			sendJSON(senderConn, Message{Type: "error", Content: "Usage: /msg <nickname> <message>"})
			return currentNick, false
		}
		sendPrivate(addr, currentNick, parts[1], parts[2], connReg)
		return currentNick, false
	}

	if command == "quit" {
		return currentNick, true
	}

	sendJSON(senderConn, Message{Type: "error", Content: "Unknown command."})
	return currentNick, false
}

func sendList(senderConn net.Conn, connReg *ClientRegistry) {
	connReg.mu.RLock()
	defer connReg.mu.RUnlock()

	var users []string
	for _, conn := range connReg.connections {
		roomDisplay := conn.room
		if roomDisplay == "" {
			roomDisplay = "none"
		}
		users = append(users, fmt.Sprintf("%s (Room: %s)", conn.nickname, roomDisplay))
	}

	sendJSON(senderConn, Message{
		Type:    "system",
		Content: "Users online:\n- " + strings.Join(users, "\n- "),
	})
}

func sendPrivate(senderAddr, senderNick, targetNick, content string, connReg *ClientRegistry) {
	connReg.mu.RLock()
	var targetConn net.Conn
	for _, c := range connReg.connections {
		if c.nickname == targetNick {
			targetConn = c.conn
			break
		}
	}
	senderConn := connReg.connections[senderAddr].conn
	connReg.mu.RUnlock()

	if targetConn != nil {
		sendJSON(targetConn, Message{
			Type:    "private",
			Sender:  senderNick,
			Target:  targetNick,
			Content: content,
		})
	} else {
		sendJSON(senderConn, Message{
			Type:    "error",
			Content: "User not found.",
		})
	}
}

func getNickname(conn net.Conn, scanner *bufio.Scanner) string {
	sendJSON(conn, Message{Type: "system", Content: "Enter nickname:"})
	if scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil {
			return strings.TrimSpace(msg.Content)
		}
	}
	return "Guest"
}

func broadcast(senderAddr string, msg Message, connReg *ClientRegistry) {
	if msg.Room == "" {
		return
	}

	connReg.mu.RLock()
	defer connReg.mu.RUnlock()

	for addr, c := range connReg.connections {
		if addr != senderAddr && c.room == msg.Room {
			sendJSON(c.conn, msg)
		}
	}
}

func handleConnection(conn net.Conn, connReg *ClientRegistry) {
	addr := conn.RemoteAddr().String()
	scanner := bufio.NewScanner(conn)

	nick := getNickname(conn, scanner)
	log.Printf("%s connected!", nick)

	connReg.mu.Lock()
	connReg.connections[addr] = Connection{
		conn:     conn,
		nickname: nick,
		room:     "",
	}
	connReg.mu.Unlock()

	defer func() {
		conn.Close()
		connReg.mu.Lock()
		c := connReg.connections[addr]
		delete(connReg.connections, addr)
		connReg.mu.Unlock()
		log.Printf("%s disconnected!", nick)
		if c.room != "" {
			broadcast(addr, Message{Type: "system", Room: c.room, Content: fmt.Sprintf("%s has left the room.", nick)}, connReg)
		}
	}()

	for scanner.Scan() {
		var msg Message
		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			sendJSON(conn, Message{Type: "error", Content: "Invalid JSON format. Expected NDJSON."})
			continue
		}

		if msg.Type == "private" {
			sendPrivate(addr, nick, msg.Target, msg.Content, connReg)
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		if strings.HasPrefix(content, "/") {
			var shouldQuit bool
			nick, shouldQuit = executeCommand(content[1:], addr, connReg, nick)
			if shouldQuit {
				return
			}
		} else if msg.Type == "chat" {
			connReg.mu.RLock()
			room := connReg.connections[addr].room
			connReg.mu.RUnlock()

			if room != "" {
				broadcast(addr, Message{
					Type:    "chat",
					Sender:  nick,
					Room:    room,
					Content: content,
				}, connReg)
			} else {
				sendJSON(conn, Message{
					Type:    "error",
					Content: "You are not in a room. Use /join <room> to chat.",
				})
			}
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", ":8080")
	connReg := ClientRegistry{
		connections: make(map[string]Connection),
	}

	if err != nil {
		log.Fatalf("Err while listening: %v", err)
	}
	log.Println("Started listening on localhost:8080")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Err while Accepting conn : ", err)
			continue
		}

		go handleConnection(conn, &connReg)
	}
}
