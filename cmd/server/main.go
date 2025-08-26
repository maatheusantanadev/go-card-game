package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

type Player struct {
	Conn net.Conn
	Name string
}

var (
	players   = make([]*Player, 0)
	playerMux sync.Mutex
)

func main() {
	fmt.Println("Iniciando jogo de cartas multiplayer...")
	Start(":5000")
}

func Start(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor rodando em", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		go handlePlayer(conn)
	}
}

func handlePlayer(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// --- Lê nome do jogador ---
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = conn.RemoteAddr().String()
	}

	player := &Player{Conn: conn, Name: name}

	// Adiciona jogador
	playerMux.Lock()
	players = append(players, player)
	playerMux.Unlock()

	fmt.Printf("Novo jogador conectado: %s\n", player.Name)

	
	defer func() {
		playerMux.Lock()
		for i, p := range players {
			if p == player {
				players = append(players[:i], players[i+1:]...)
				break
			}
		}
		playerMux.Unlock()
		fmt.Printf("Jogador desconectado: %s\n", player.Name)
	}()

	
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		fmt.Printf("[%s] %s\n", player.Name, message)

		
		playerMux.Lock()
		for _, p := range players {
			if p != player {
				p.Conn.Write([]byte(fmt.Sprintf("[%s] %s\n", player.Name, message)))
			}
		}
		playerMux.Unlock()
	}
}
