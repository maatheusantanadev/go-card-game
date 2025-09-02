package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"encoding/json"
)


type Player struct {
	Conn net.Conn
	Name string
}

type Card struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Rarity   string `json:"rarity"`
    Power    int    `json:"power"`
}

type GameState struct {
    Players   [2]*Player
    Decks     map[string][]Card // chave: nome do jogador
    Hands     map[string][]Card
    Turn      int               // índice do jogador da vez (0 ou 1)
    Started   bool
}

type GameAction struct {
    Action string `json:"action"` // play_card, end_turn, draw_booster
    CardID int    `json:"card_id,omitempty"`
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
        return "❌ Carta não encontrada na sua mão."

    case "end_turn":
        state.Turn = (state.Turn + 1) % 2
        next := state.Players[state.Turn]
        // avisa que o turno mudou
        msg := fmt.Sprintf("🔄 Turno passou para %s", next.Name)
        // mostra a mão do próximo jogador
        showHand(next, state)
        return msg

    case "draw_booster":
        newCard := Card{ID: 99, Name: "Carta Rara", Rarity: "Rare", Power: 5}
        state.Hands[player.Name] = append(state.Hands[player.Name], newCard)
        return fmt.Sprintf("📦 %s abriu um booster e recebeu %s!", player.Name, newCard.Name)

    default:
        return "❌ Ação desconhecida."
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
	ln, _ := net.Listen("tcp", ":0") // cria porta TCP livre para sala
	addr := ln.Addr().String()

	// também cria servidor UDP em porta livre
	udpAddr, _ := net.ResolveUDPAddr("udp", ":0")
	udpConn, _ := net.ListenUDP("udp", udpAddr)
	go udpServer(udpConn) // inicia servidor UDP paralelo

	fmt.Printf("🎮 Criando sala 1vs1 em %s (UDP %s) (%s vs %s)\n",
		addr, udpConn.LocalAddr().String(), p1.Name, p2.Name)

	// informa os jogadores sobre a sala TCP e UDP
	p1.Conn.Write([]byte("Sala TCP: " + addr + "\n"))
	p1.Conn.Write([]byte("Ping UDP: " + udpConn.LocalAddr().String() + "\n"))
	p2.Conn.Write([]byte("Sala TCP: " + addr + "\n"))
	p2.Conn.Write([]byte("Ping UDP: " + udpConn.LocalAddr().String() + "\n"))

	// fecha conexões com o lobby
	p1.Conn.Close()
	p2.Conn.Close()

	// espera 2 conexões TCP para a partida
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
    state := &GameState{
        Players: [2]*Player{p1, p2},
        Decks:   map[string][]Card{},
        Hands:   map[string][]Card{},
        Turn:    0,
        Started: true,
    }

    // decks de exemplo
    state.Decks[p1.Name] = []Card{
        {ID: 1, Name: "Espadachim", Rarity: "Common", Power: 2},
        {ID: 2, Name: "Mago", Rarity: "Rare", Power: 4},
        {ID: 5, Name: "Clérigo", Rarity: "Common", Power: 3},
    }
    state.Decks[p2.Name] = []Card{
        {ID: 3, Name: "Arqueira", Rarity: "Common", Power: 3},
        {ID: 4, Name: "Dragão", Rarity: "Epic", Power: 6},
        {ID: 6, Name: "Guerreiro", Rarity: "Common", Power: 2},
    }

    // cada jogador começa com 2 cartas (se houver no deck)
    state.Hands[p1.Name] = state.Decks[p1.Name][:2]
    state.Hands[p2.Name] = state.Decks[p2.Name][:2]

    go relay(p1, p2, state)
    go relay(p2, p1, state)

    // avisa quem começa
    for _, p := range state.Players {
        p.Conn.Write([]byte("✅ Partida iniciada! É a vez de " + p1.Name + "\n"))
    }

    // mostra a mão do jogador inicial
    showHand(state.Players[state.Turn], state)
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
        sb.WriteString(fmt.Sprintf("  [%d] %s (Poder %d, %s)\n", c.ID, c.Name, c.Power, c.Rarity))
    }
    player.Conn.Write([]byte(sb.String()))
}

func relay(src, dst *Player, state *GameState) {
    reader := bufio.NewReader(src.Conn)
    for {
        msg, err := reader.ReadString('\n')
        if err != nil {
            fmt.Printf("⚠ Jogador %s se desconectou\n", src.Name)
            dst.Conn.Write([]byte(src.Name + " saiu da partida.\n"))
            src.Conn.Close()
            return
        }

        msg = strings.TrimSpace(msg)

        var action GameAction
        if err := json.Unmarshal([]byte(msg), &action); err == nil {
            // é uma ação de jogo
            response := handleAction(state, src, action)
            for _, p := range state.Players {
                p.Conn.Write([]byte(response + "\n"))
            }
        } else {
            // é só mensagem de chat
            dst.Conn.Write([]byte(fmt.Sprintf("[%s] %s\n", src.Name, msg)))
        }
    }
}

func udpServer(conn *net.UDPConn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("⚠ Erro no UDP:", err)
			return
		}
		msg := strings.TrimSpace(string(buf[:n]))
		if msg == "ping" {
			conn.WriteToUDP([]byte("pong"), clientAddr)
		}
	}
}

