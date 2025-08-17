package main

import (
	"fmt"
	"github.com/maatheusantanadev/go-card-game/internal/server"
)

func main() {
	fmt.Println("Iniciando servidor de cartas multiplayer...")
	server.Start(":5000")
}