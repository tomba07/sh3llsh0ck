# sh3llsh0ck

A multiplayer terminal trap game. Move around a maze, place traps, and score points by catching other players in the blast. Traps chain-detonate when caught in each other's radius.

```
  > Host a game
    Join a game
```

## Gameplay

| Key | Action |
|-----|--------|
| Arrow keys | Move |
| Space | Place trap |
| `q` | Quit |

- Each player can have up to **3 active traps** at a time.
- A trap detonates after **2 seconds**, producing a cross-shaped blast with radius 2.
- Blasts that overlap another trap trigger a **chain reaction**.
- Hit players respawn at the furthest available spawn point.
- The top 3 scores are shown in a leaderboard to the right of the map.

## Running

```sh
go run ./cmd/sh3llsh0ck
```

Choose **Host a game** to start a server on port 50051 — your LAN address is printed so others can join. Choose **Join a game** and enter `<host-ip>:50051`.

## Architecture

The game runs as a single binary that acts as either server, client, or both:

- **Server** (`server.go`) — gRPC service managing game state, trap scheduling, and chain-reaction resolution. Players are identified by name.
- **Client** (`client.go`) — subscribes to a server-pushed event stream and renders the map to the terminal using ANSI escape codes.
- **Game logic** (`game.go`) — pure state: map layout, spawning, trap placement, and blast/chain logic.
- **Transport** — [gRPC](https://grpc.io/) with protobuf (`proto/chat.proto`).

## Regenerating the proto

Requires `protoc` with the Go and gRPC plugins.

```sh
make proto
```
