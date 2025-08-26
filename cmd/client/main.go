package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

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

	// --- envia nome ---
	fmt.Print("Digite seu nome: ")
	name, _ := stdin.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Jogador"
	}
	conn.Write([]byte(name + "\n"))

	// --- espera resposta do lobby ---
	serverReader := bufio.NewReader(conn)
	line, _ := serverReader.ReadString('\n')
	fmt.Print(line)

	// se foi criada uma sala, pega o endereço e reconecta
	if strings.HasPrefix(line, "Sala criada em:") {
		addr := strings.TrimSpace(strings.TrimPrefix(line, "Sala criada em:"))
		conn.Close()
		playInMatch(addr, name)
	} else {
		// aguarda até receber sala
		for {
			line, err = serverReader.ReadString('\n')
			if err != nil {
				return
			}
			fmt.Print(line)
			if strings.HasPrefix(line, "Sala criada em:") {
				addr := strings.TrimSpace(strings.TrimPrefix(line, "Sala criada em:"))
				conn.Close()
				playInMatch(addr, name)
				return
			}
		}
	}
}

func playInMatch(addr, name string) {
	fmt.Println("🔗 Conectando na sala", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Erro ao conectar na sala:", err)
		return
	}
	defer conn.Close()

	// envia novamente o nome
	conn.Write([]byte(name + "\n"))

	// goroutine para ouvir servidor
	go func() {
		serverReader := bufio.NewScanner(conn)
		for serverReader.Scan() {
			fmt.Printf("\r%s\nVocê: ", serverReader.Text())
		}
	}()

	// loop de envio
	stdin := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Você: ")
		text, _ := stdin.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		conn.Write([]byte(text + "\n"))
	}
}
