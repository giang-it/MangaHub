package main

import "mangahub/internal/tcp"

func main() {
	server := tcp.NewProgressSyncServer("8081")
	server.Start()
}
