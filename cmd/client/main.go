// client.go
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
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

		if cmd == "/ping" {
        // mede latência usando UDP
        latencia := medirPingUDP("localhost:4001")
        if latencia > 0 {
            fmt.Printf("Servidor: Latência UDP = %d ms\n", latencia.Milliseconds())
        } else {
            fmt.Println("Servidor: ping falhou")
        }
        continue
    	}

		// Envia comando ou JSON de ação para o servidor
		_, err := conn.Write([]byte(cmd + "\n"))
		if err != nil {
			log.Println("Erro ao enviar:", err)
			return
		}
	}
}

// medirPingUDP envia um pacote UDP "ping" e espera por "pong", retornando a latência
func medirPingUDP(endereco string) time.Duration {
    conn, err := net.Dial("udp", endereco)
    if err != nil {
        log.Println("Erro UDP:", err)
        return 0
    }
    defer conn.Close()

    inicio := time.Now()
    _, err = conn.Write([]byte("ping"))
    if err != nil {
        log.Println("Erro ao enviar ping:", err)
        return 0
    }

    buf := make([]byte, 16)
    conn.SetReadDeadline(time.Now().Add(2 * time.Second)) // timeout
    n, err := conn.Read(buf)
    if err != nil {
        log.Println("Erro ao ler resposta UDP:", err)
        return 0
    }

    resposta := strings.TrimSpace(string(buf[:n]))
    if resposta == "pong" {
        return time.Since(inicio)
    }
    return 0
}


