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

var pingChan = make(chan string, 1) // canal para atualizar latência

// igual ao servidor
type GameAction struct {
	Action string `json:"action"`
	CardID int    `json:"card_id,omitempty"`
}

func main() {
	fmt.Println("Iniciando jogo de cartas multiplayer...")

	// conecta no lobby
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

	var salaTCP, salaUDP string

	for {
		line, err := serverReader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		fmt.Println(line)

		if strings.HasPrefix(line, "Sala TCP:") {
			salaTCP = strings.TrimSpace(strings.TrimPrefix(line, "Sala TCP:"))
		}
		if strings.HasPrefix(line, "Ping UDP:") {
			salaUDP = strings.TrimSpace(strings.TrimPrefix(line, "Ping UDP:"))
		}

		// quando já temos ambos os endereços → conecta na partida
		if salaTCP != "" && salaUDP != "" {
			conn.Close()
			playInMatch(salaTCP, salaUDP, name)
			return
		}
	}
}

func playInMatch(addrTCP, addrUDP, name string) {
	fmt.Println("🔗 sala:", addrTCP)
	conn, err := net.Dial("tcp", addrTCP)
	if err != nil {
		fmt.Println("Erro ao conectar na sala:", err)
		return
	}
	defer conn.Close()

	// envia novamente o nome
	conn.Write([]byte(name + "\n"))

	// goroutine para ouvir servidor TCP (chat + respostas do jogo)
	go func() {
		serverReader := bufio.NewScanner(conn)
		for serverReader.Scan() {
			fmt.Printf("\r%s\nVocê: ", serverReader.Text())
		}
	}()

	// goroutine para medir latência UDP
	go pingUDP(addrUDP)

	// goroutine para exibir latência atualizada
	go func() {
		for ping := range pingChan {
			fmt.Printf("\r%s\nVocê: ", ping)
		}
	}()

	// loop de envio de mensagens (comandos ou chat)
	stdin := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Você: ")
		text, _ := stdin.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// ----------------------
		// comandos de jogo
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
		time.Sleep(40 * time.Second)
	}
}
