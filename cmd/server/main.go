package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Player struct {
	Conn net.Conn
	Name string
}

var waiting *Player     // jogador aguardando
var matchMux sync.Mutex // trava para acessar "waiting"

func main() {
	listener, _ := net.Listen("tcp", ":4000")
	fmt.Println("Lobby rodando em :4000")

	for {
		conn, _ := listener.Accept()
		go handleLobby(conn)
	}
}

func handleLobby(conn net.Conn) {
	reader := bufio.NewReader(conn)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = conn.RemoteAddr().String()
	}
	player := &Player{Conn: conn, Name: name}

	matchMux.Lock()
	if waiting == nil {
		// primeiro jogador fica esperando
		waiting = player
		player.Conn.Write([]byte("⏳ Aguardando outro jogador...\n"))
		matchMux.Unlock()
	} else {
		// já tinha alguém esperando → inicia partida
		p1 := waiting
		waiting = nil
		matchMux.Unlock()

		go startMatch(p1, player)
	}
}

func startMatch(p1, p2 *Player) {
	ln, _ := net.Listen("tcp", ":0") // cria porta livre para a sala
	addr := ln.Addr().String()
	fmt.Printf("🎮 Criando sala 1vs1 em %s (%s vs %s)\n", addr, p1.Name, p2.Name)

	// informa os jogadores sobre a sala
	p1.Conn.Write([]byte("Sala criada em: " + addr + "\n"))
	p2.Conn.Write([]byte("Sala criada em: " + addr + "\n"))

	// fecha conexões com o lobby
	p1.Conn.Close()
	p2.Conn.Close()

	// espera exatamente 2 conexões na nova porta
	players := []*Player{}
	for len(players) < 2 {
		conn, _ := ln.Accept()
		reader := bufio.NewReader(conn)
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if name == "" {
			name = conn.RemoteAddr().String()
		}
		players = append(players, &Player{Conn: conn, Name: name})
		fmt.Println("➡ Jogador entrou na sala:", name)
	}

	fmt.Printf("✅ Sala iniciada: %s vs %s\n", players[0].Name, players[1].Name)
	runMatch(players[0], players[1])
}

func runMatch(p1, p2 *Player) {
	// cada mensagem/jogada é repassada ao adversário
	go relay(p1, p2)
	go relay(p2, p1)
}

func relay(src, dst *Player) {
	reader := bufio.NewReader(src.Conn)
	for {
		src.Conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // timeout
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("⚠ Jogador %s se desconectou\n", src.Name)
			dst.Conn.Write([]byte(src.Name + " saiu da partida.\n"))
			src.Conn.Close()
			return
		}
		dst.Conn.Write([]byte(fmt.Sprintf("[%s] %s", src.Name, msg)))
	}
}
