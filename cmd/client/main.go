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

func main() {
	// Conexão TCP com o servidor
	conn, err := net.Dial("tcp", "localhost:4000")
	if err != nil {
		log.Fatal("Erro ao conectar TCP:", err)
	}
	defer conn.Close()

	fmt.Println("Conectado ao servidor TCP em :4000")

	// Leitura assíncrona (mensagens do servidor)
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

	// Enviar nome ao servidor (primeira linha obrigatória)
	fmt.Print("Digite seu nome: ")
	stdin := bufio.NewReader(os.Stdin)
	name, _ := stdin.ReadString('\n')
	conn.Write([]byte(name))

	// Loop para comandos do jogador
	for {
		fmt.Print("> ")
		cmd, _ := stdin.ReadString('\n')
		cmd = strings.TrimSpace(cmd)

		if cmd == "/exit" {
			fmt.Println("Saindo...")
			return
		}

		// envia comando ou JSON de ação para o servidor
		_, err := conn.Write([]byte(cmd + "\n"))
		if err != nil {
			log.Println("Erro ao enviar:", err)
			return
		}
	}
}
