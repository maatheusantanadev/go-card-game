package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Player struct {
	Conn   net.Conn
	Name   string
	RoomID string
}

type Card struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Rarity string `json:"rarity"`
	Power  int    `json:"power"`
}

type GameState struct {
	Players [2]*Player
	Decks   map[string][]Card
	Hands   map[string][]Card
	Turn    int
	Started bool
	mu      sync.Mutex
}

type GameAction struct {
	Action string `json:"action"`
	CardID int    `json:"card_id,omitempty"`
}

var (
	waiting       *Player
	matchMux      sync.Mutex
	activeMatches = make(map[string]*GameState) // map[roomID]GameState
	udpConn       *net.UDPConn
)

func main() {
	// TCP: lobby + partidas
	tcpListener, _ := net.Listen("tcp", ":4000")
	fmt.Println("Servidor TCP rodando em :4000")

	// UDP: latência
	udpAddr, _ := net.ResolveUDPAddr("udp", ":5000")
	udpConn, _ = net.ListenUDP("udp", udpAddr)
	fmt.Println("Servidor UDP rodando em :5000")
	go udpServer(udpConn)

	for {
		conn, _ := tcpListener.Accept()
		go handleTCP(conn)
	}
}

// ================= TCP =================

func handleTCP(conn net.Conn) {
	reader := bufio.NewReader(conn)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = conn.RemoteAddr().String()
	}
	player := &Player{Conn: conn, Name: name}

	// Mutex para entrada de partidas
	matchMux.Lock()
	if waiting == nil {
		waiting = player
		player.Conn.Write([]byte("⏳ Aguardando outro jogador...\n"))
		matchMux.Unlock()
	} else {
		p1 := waiting
		waiting = nil
		matchMux.Unlock()

		roomID := fmt.Sprintf("%d", time.Now().UnixNano())
		game := &GameState{
			Players: [2]*Player{p1, player},
			Decks:   map[string][]Card{},
			Hands:   map[string][]Card{},
			Turn:    0,
			Started: true,
		}

		activeMatches[roomID] = game
		p1.RoomID = roomID
		player.RoomID = roomID

		startMatch(game)
	}
}

func startMatch(state *GameState) {
	p1 := state.Players[0]
	p2 := state.Players[1]

	// decks iniciais
	state.Decks[p1.Name] = []Card{
		{ID: 1, Name: "Espadachim", Rarity: "Common", Power: 2},
		{ID: 2, Name: "Mago", Rarity: "Rare", Power: 4},
	}
	state.Decks[p2.Name] = []Card{
		{ID: 3, Name: "Arqueira", Rarity: "Common", Power: 3},
		{ID: 4, Name: "Dragão", Rarity: "Epic", Power: 6},
	}

	state.Hands[p1.Name] = state.Decks[p1.Name][:1]
	state.Hands[p2.Name] = state.Decks[p2.Name][:1]

	for _, p := range state.Players {
		p.Conn.Write([]byte(fmt.Sprintf("✅ Partida iniciada! Sala: %s\n", p.RoomID)))
        p.Conn.Write([]byte("Ping UDP: localhost:5000\n"))
		showHand(p, state)
	}

	for _, p := range state.Players {
		go relay(p, state)
	}
}

func relay(player *Player, state *GameState) {
	reader := bufio.NewReader(player.Conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("⚠ Jogador %s se desconectou\n", player.Name)
			return
		}
		msg = strings.TrimSpace(msg)

		var action GameAction
		if err := json.Unmarshal([]byte(msg), &action); err == nil {
			state.mu.Lock() // mutex para concorrência no jogo
			response := handleAction(state, player, action)
			state.mu.Unlock()
			for _, p := range state.Players {
				p.Conn.Write([]byte(response + "\n"))
			}
		} else {
			// chat simples
			for _, p := range state.Players {
				if p.Name != player.Name {
					p.Conn.Write([]byte(fmt.Sprintf("[%s] %s\n", player.Name, msg)))
				}
			}
		}
	}
}

func handleAction(state *GameState, player *Player, action GameAction) string {
	current := state.Players[state.Turn]
	if player.Name != current.Name {
		return "❌ Não é sua vez."
	}

	switch action.Action {
	case "play_card":
		hand := state.Hands[player.Name]
		for i, card := range hand {
			if card.ID == action.CardID {
				state.Hands[player.Name] = append(hand[:i], hand[i+1:]...)
				return fmt.Sprintf("🃏 %s jogou %s!", player.Name, card.Name)
			}
		}
		return "❌ Carta não encontrada."
	case "end_turn":
		state.Turn = (state.Turn + 1) % 2
		next := state.Players[state.Turn]
		showHand(next, state)
		return fmt.Sprintf("🔄 Turno passou para %s", next.Name)
	case "draw_booster":
		// Mutex para concorrência na compra de pacotes
		state.mu.Lock()
		defer state.mu.Unlock()
		newCard := Card{ID: 99, Name: "Carta Rara", Rarity: "Legend", Power: 5}
		state.Hands[player.Name] = append(state.Hands[player.Name], newCard)
		return fmt.Sprintf("📦 %s abriu um booster e recebeu %s!", player.Name, newCard.Name)
	default:
		return "❌ Ação desconhecida."
	}
}

func showHand(player *Player, state *GameState) {
	hand := state.Hands[player.Name]
	if len(hand) == 0 {
		player.Conn.Write([]byte("🖐️ Sua mão está vazia.\n"))
		return
	}
	var sb strings.Builder
	sb.WriteString("🖐️ Sua mão:\n")
	for _, c := range hand {
		sb.WriteString(fmt.Sprintf("  [%d] %s (%s, Poder %d)\n", c.ID, c.Name, c.Rarity, c.Power))
	}
	player.Conn.Write([]byte(sb.String()))
}

// ================= UDP =================

func udpServer(conn *net.UDPConn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("⚠ Erro no UDP:", err)
			continue
		}
		msg := strings.TrimSpace(string(buf[:n]))
		if msg == "ping" {
			conn.WriteToUDP([]byte("pong"), clientAddr)
		}
	}
}
