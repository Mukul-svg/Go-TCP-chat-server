# TCP Chat Server (Go)

I built this project mainly for learning Go concurrency and networking in a practical way.

It's a simple chat server where multiple clients can connect, chat in rooms, send private messages, and manage their nicknames. Different clients can join the same room and see each other's messages in real time.

For me, this project is less about building a perfect chat system and more about understanding how goroutines, channels, mutexes, and TCP connections work together in real code.

## Why I made this

I wanted to learn these things by building something real instead of solving isolated examples:

- How to handle multiple concurrent TCP clients safely
- How to synchronize shared state across goroutines
- How to structure messages and protocol design for a server
- How to manage client connections and graceful disconnects
- How to implement chat rooms and private messaging

## What this server does

- Accepts multiple TCP connections on port 8080
- Asks each client for a nickname when they connect
- Lets clients join chat rooms and broadcast messages to others in the same room
- Sends private messages between clients
- Handles commands like `/nick`, `/join`, `/leave`, `/list`, and `/msg`
- Logs client connections and disconnections
- Removes clients cleanly when they disconnect

## What I learned

- Thread safety is critical even in learning projects. A shared client registry needs proper locking.
- JSON over TCP works well for simple protocols, but reading line-by-line (NDJSON) takes some care.
- Goroutines handle concurrent clients easily, but you need to think about cleanup when they exit.
- Broadcast logic to rooms needs to check both sender address and room membership.
- Unexpected things can happen with network reads. Error handling matters.

## Requirements

- Go 1.26+ (or any recent Go version)

## Run

```bash
go run main.go
```

This starts the server on `localhost:8080`. Clients can then connect using any TCP client (like `nc`, Telnet, or a custom client).

## Commands

When connected, clients can send commands. Send all messages as JSON, one per line:

| Command | Example | What it does |
|---------|---------|------|
| Set nickname | `{"type":"system","content":"/nick alice"}` | Changes your nickname |
| Join room | `{"type":"system","content":"/join general"}` | Join a chat room |
| Leave room | `{"type":"system","content":"/leave"}` | Leave your current room |
| List users | `{"type":"system","content":"/list"}` | See all connected users and their rooms |
| Send chat | `{"type":"chat","content":"hello"}` | Send a message to your current room |
| Private msg | `{"type":"private","target":"bob","content":"hey"}` | Send a private message to another user |
| Quit | `{"type":"system","content":"/quit"}` | Disconnect |

## How it works (quick view)

1. Server listens on port 8080.
2. When a client connects, it asks for their nickname.
3. The client's connection is stored in a registry with their nickname and current room.
4. Clients send messages as JSON lines (newline-delimited).
5. Commands starting with `/` are parsed and executed.
6. Chat messages are broadcast to all clients in the same room.
7. Private messages are sent directly to a specific client by nickname.
8. When a client disconnects, they're removed from the registry and others in their room are notified.

## Why I made these design choices

- **TCP instead of HTTP**: Direct socket communication is simpler for real-time messaging.
- **JSON over NDJSON**: Each message is one line of JSON, easy to parse with bufio.Scanner.
- **ClientRegistry with mutex**: A simple map is easy to understand and good enough for this scale.
- **Goroutine per client**: Each connection runs in its own goroutine, they don't block each other.
- **Commands with `/` prefix**: Easy to distinguish between chat and commands.
- **Rooms as strings**: No room objects, just a string field. Simple and sufficient.

## Challenges I faced

- Getting locking right everywhere. Forgetting a lock in one place causes subtle race conditions.
- Deciding when to lock the registry. Read locks are fine for checking, but writes need full locks.
- Handling client disconnect while they're in the middle of sending a message.
- Broadcasting to exactly the right people. Can't forget to exclude the sender.
- Figuring out the protocol format. JSON made it simple, but NDJSON parsing needs care.

## Notes

- This is a learning project. It's not production-ready.
- There's no authentication or encryption.
- No persistence. Messages disappear when the server stops.
- Room names are case-sensitive.
- No input validation on nickname or room names.

## Possible next improvements

- Add authentication (username/password)
- Save chat history to a file or database
- Add TLS/SSL for encrypted connections
- Implement read timeouts to clean up stuck clients
- Add a Web UI as alternative to raw JSON commands
- Add user statuses (online, idle, busy)
- Implement message history when joining a room
- Add rate limiting to prevent spam
