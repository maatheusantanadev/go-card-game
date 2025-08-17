package server

import (
    "bufio"
    "fmt"
    "net"
    "sync"
)

var (
    players   = make([]net.Conn, 0)
    playerMux sync.Mutex
)

func Start(address string) {
    listener, err := net.Listen("tcp", address)
    if err != nil {
        fmt.Println("Erro ao iniciar servidor:", err)
        return
    }
    defer listener.Close()

    fmt.Println("Servidor rodando em", address)

    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("Erro ao aceitar conexão:", err)
            continue
        }
        fmt.Println("Novo jogador conectado:", conn.RemoteAddr())
        go handlePlayer(conn)
    }
}

func handlePlayer(conn net.Conn) {
    defer conn.Close()

    playerMux.Lock()
    players = append(players, conn)
    playerMux.Unlock()

    reader := bufio.NewReader(conn)
    for {
        message, err := reader.ReadString('\n')
        if err != nil {
            fmt.Println("Jogador desconectado:", conn.RemoteAddr())
            break
        }
        fmt.Printf("[%s] %s", conn.RemoteAddr(), message)
        conn.Write([]byte("Servidor recebeu: " + message))
    }
}
