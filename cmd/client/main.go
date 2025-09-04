package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var pingChan = make(chan string, 1)

// igual ao servidor
type GameAction struct {
	Action string `json:"action"`
	CardID int    `json:"card_id,omitempty"`
}

func main() {
	fmt.Println("Iniciando jogo de cartas multiplayer...")

	// conecta no servidor principal
	conn, err := net.Dial("tcp", "localhost:4000")
	if err != nil {
		fmt.Println("Erro ao conectar no lobby:", err)
		return
	}
	defer conn.Close()

	stdin := bufio.NewReader(os.Stdin)

	// envia nome
	fmt.Print("Digite seu nome: ")
	name, _ := stdin.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Jogador"
	}
	conn.Write([]byte(name + "\n"))

	// aguarda resposta do lobby
	serverReader := bufio.NewReader(conn)

	var salaUDP string

	// goroutine para ouvir servidor (chat + respostas do jogo)
	go func() {
		for {
			line, err := serverReader.ReadString('\n')
			if err != nil {
				fmt.Println("⚠ Conexão encerrada pelo servidor.")
				os.Exit(0)
			}
			line = strings.TrimSpace(line)
			fmt.Printf("\r%s\nVocê: ", line)

			if strings.HasPrefix(line, "Ping UDP:") {
				salaUDP = strings.TrimSpace(strings.TrimPrefix(line, "Ping UDP:"))
				go pingUDP(salaUDP)
			}
		}
	}()

	// loop de envio de mensagens (comandos ou chat)
	for {
		fmt.Print("Você: ")
		text, _ := stdin.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// ----------------------
		// comandos de jogo
		if text == "/ping" {
			fmt.Println(getPing(salaUDP))
			continue	
	}

		if strings.HasPrefix(text, "/play") {
			parts := strings.Split(text, " ")
			if len(parts) == 2 {
				cardID, _ := strconv.Atoi(parts[1])
				action := GameAction{Action: "play_card", CardID: cardID}
				sendAction(conn, action)
			}
			continue
		}
		if text == "/end" {
			action := GameAction{Action: "end_turn"}
			sendAction(conn, action)
			continue
		}
		if text == "/booster" {
			action := GameAction{Action: "draw_booster"}
			sendAction(conn, action)
			continue
		}

		// ----------------------
		// chat normal
		conn.Write([]byte(text + "\n"))
	}
}

func sendAction(conn net.Conn, action GameAction) {
	data, _ := json.Marshal(action)
	conn.Write(append(data, '\n'))
}

// envia pings via UDP e mede RTT
func pingUDP(addr string) {
	serverAddr, _ := net.ResolveUDPAddr("udp", addr)
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Println("Erro no UDP:", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		start := time.Now()
		conn.Write([]byte("ping"))

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			pingChan <- "🏓 Latência: timeout"
			time.Sleep(1 * time.Second)
			continue
		}

		rtt := time.Since(start)
		pingChan <- fmt.Sprintf("🏓 Latência UDP: %v", rtt.Round(time.Millisecond))
		time.Sleep(5 * time.Second)
	}
}

func getPing(addr string) string {
    serverAddr, _ := net.ResolveUDPAddr("udp", addr)
    conn, err := net.DialUDP("udp", nil, serverAddr)
    if err != nil {
        return "Erro no UDP: " + err.Error()
    }
    defer conn.Close()

    start := time.Now()
    conn.Write([]byte("ping"))
    conn.SetReadDeadline(time.Now().Add(2 * time.Second))

    buf := make([]byte, 1024)
    _, _, err = conn.ReadFromUDP(buf)
    if err != nil {
        return "🏓 Latência: timeout"
    }

    rtt := time.Since(start)
    return fmt.Sprintf("🏓 Latência UDP: %v", rtt.Round(time.Millisecond))
}

