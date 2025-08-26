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

	conn, err := net.Dial("tcp", "localhost:5000")
	if err != nil {
		fmt.Println("Erro ao conectar:", err)
		return
	}
	defer conn.Close()

	// --- Lê nome do jogador ---
	fmt.Print("Digite seu nome: ")
	stdin := bufio.NewReader(os.Stdin)
	name, _ := stdin.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Jogador"
	}
	conn.Write([]byte(name + "\n"))

	// --- Goroutine para ouvir servidor ---
	go func() {
		serverReader := bufio.NewScanner(conn)
		for serverReader.Scan() {
			fmt.Printf("\r%s\nVocê: ", serverReader.Text())
		}
	}()

	// --- Loop para enviar mensagens ---
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
