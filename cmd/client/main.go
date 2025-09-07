// client.go
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

// ponto de entrada do cliente, conecta ao servidor TCP e gerencia envio/recebimento de mensagens
func main() {
	// Conexão TCP com o servidor
	conn, err := net.Dial("tcp", "localhost:4000")
	if err != nil {
		log.Fatal("Erro ao conectar TCP:", err)
	}
	defer conn.Close()

	fmt.Println("Conectado ao servidor TCP em :4000")

	// Goroutine: leitura assíncrona de mensagens enviadas pelo servidor
	go func() {
		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Desconectado do servidor:", err)
				return
			}
			fmt.Print("Servidor: " + msg)
		}
	}()

	// Envia o nome do jogador ao servidor 
	fmt.Print("Digite seu nome: ")
	stdin := bufio.NewReader(os.Stdin)
	name, _ := stdin.ReadString('\n')
	conn.Write([]byte(name))

	// captura comandos do jogador e envia para o servidor
	for {
		fmt.Print("> ")
		cmd, _ := stdin.ReadString('\n')
		cmd = strings.TrimSpace(cmd)

		if cmd == "/exit" {
			fmt.Println("Saindo...")
			return
		}

		// Envia comando ou JSON de ação para o servidor
		_, err := conn.Write([]byte(cmd + "\n"))
		if err != nil {
			log.Println("Erro ao enviar:", err)
			return
		}
	}
}
