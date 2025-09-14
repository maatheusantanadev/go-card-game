
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	serverAddr = flag.String("addr", "localhost:4000", "endereço do servidor TCP")
	clients    = flag.Int("clients",300, "número de clientes simultâneos")
	duration   = flag.Duration("duration", 60*time.Second, "duração do teste")
)

type stats struct {
	successConns int64
	failConns    int64
	actionsSent  int64
	actionsErr   int64
	latencies    []time.Duration
	winCount     int64
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	st := &stats{}
	var latMu sync.Mutex

	stopAt := time.Now().Add(*duration)

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nome := fmt.Sprintf("LoadBot-%d", id)
			conn, err := net.Dial("tcp", *serverAddr)
			if err != nil {
				atomic.AddInt64(&st.failConns, 1)
				return
			}
			atomic.AddInt64(&st.successConns, 1)
			defer conn.Close()

			// leitor que captura mensagens do servidor
			reader := bufio.NewReader(conn)
			// envia nome
			_, _ = conn.Write([]byte(nome + "\n"))

			// start goroutine de leitura para detectar "vencedor" e medir latência via eco de mensagens
			msgCh := make(chan string, 100)
			go func() {
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						close(msgCh)
						return
					}
					msgCh <- strings.TrimSpace(line)
				}
			}()

			// entra na fila
			_, _ = conn.Write([]byte("/entrar\n"))

			// loop de ações até stopAt
			for time.Now().Before(stopAt) {
				// espera um tempo aleatório entre ações para simular jogadores humanos
				time.Sleep(time.Duration(200+rand.Intn(800)) * time.Millisecond)

				// pega um comando: 70% /mao, 30% jogar carta se tiver id simples
				var cmd string
				if rand.Float64() < 0.7 {
					cmd = "/mao"
				} else {
					// tenta jogar id aleatório entre 1 e 20
					card := 1 + rand.Intn(20)
					cmd = fmt.Sprintf("/jogar %d", card)
				}

				start := time.Now()
				_, err := conn.Write([]byte(cmd + "\n"))
				atomic.AddInt64(&st.actionsSent, 1)
				if err != nil {
					atomic.AddInt64(&st.actionsErr, 1)
					return
				}

				// espera por qualquer resposta do servidor ou timeout
				select {
				case m, ok := <-msgCh:
					if !ok {
						return
					}
					// simples heurística: se a mensagem contém "venceu" conta como vitória
					if strings.Contains(strings.ToLower(m), "venceu") {
						atomic.AddInt64(&st.winCount, 1)
					}
					lat := time.Since(start)
					latMu.Lock()
					st.latencies = append(st.latencies, lat)
					latMu.Unlock()
				case <-time.After(2 * time.Second):
					atomic.AddInt64(&st.actionsErr, 1)
				}
			}
		}(i)
	}

	// esperar goroutines terminarem
	wg.Wait()

	// compila estatísticas
	fmt.Println("=== Resultado do Load Test ===")
	fmt.Printf("Clientes requisitados: %d\n", *clients)
	fmt.Printf("Conexões bem-sucedidas: %d\n", st.successConns)
	fmt.Printf("Conexões falhas: %d\n", st.failConns)
	fmt.Printf("Ações enviadas: %d (erros: %d)\n", st.actionsSent, st.actionsErr)
	fmt.Printf("Vitórias detectadas: %d\n", st.winCount)

	// latências
	latMu.Lock()
	lat := st.latencies
	latMu.Unlock()
	if len(lat) == 0 {
		fmt.Println("Nenhuma latência registrada")
		return
	}
	// calcula p50 p90 p99
	sortDur := func(a []time.Duration) {
		for i := 1; i < len(a); i++ {
			key := a[i]
			j := i - 1
			for j >= 0 && a[j] > key {
				a[j+1] = a[j]
				j--
			}
			a[j+1] = key
		}
	}
	sortDur(lat)
	getPct := func(p float64) time.Duration {
		idx := int(float64(len(lat)) * p / 100.0)
		if idx >= len(lat) {
			idx = len(lat) - 1
		}
		return lat[idx]
	}
	total := time.Duration(0)
	for _, d := range lat { total += d }
	avg := total / time.Duration(len(lat))

	fmt.Printf("Latências registradas: %d\n", len(lat))
	fmt.Printf("avg: %s, p50: %s, p90: %s, p99: %s\n",
		avg, getPct(50), getPct(90), getPct(99))
}
