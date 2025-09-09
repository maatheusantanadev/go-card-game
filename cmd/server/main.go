// servidor.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// representa uma ação enviada pelo jogador (JSON)
type AcaoJogo struct {
	Acao   string `json:"acao"`          // tipo da ação, ex: "jogar_carta", "fim_turno"
	CartaID int   `json:"carta_id,omitempty"` // id da carta, se aplicável
}

// representa um jogador conectado
type Jogador struct {
	ID        string
	Nome      string
	Conexao   net.Conn      // conexão TCP com o jogador
	Saida     chan string   // canal para enviar mensagens ao jogador
	EmPartida bool          // se está em uma partida
	EnderecoUDP string       // endereço UDP do jogador (para ping)
	mu        sync.Mutex    // mutex para proteger campos como EmPartida
	UltimoPing time.Duration // último ping registrado
}

// representa uma partida entre dois jogadores
type Partida struct {
	ID      string
	A, B    *Jogador        // jogadores da partida
	Turno   string           // ID do jogador que tem a vez
	Criada  time.Time        // timestamp da criação
	mu      sync.Mutex       // mutex para proteger o estado da partida
	Mao     map[string][]int // cartas na mão dos jogadores (ID jogador -> cartas)
	Vida    map[string]int   // vida dos jogadores (ID jogador -> vida)
}

// representa um pacote booster de cartas
type PacoteBooster struct {
	ID    string
	Cartas []string // lista de cartas do pacote
}

// Variáveis globais do servidor
var (
	jogadoresMu sync.Mutex
	jogadores   = map[string]*Jogador{} // mapa de ID -> jogador

	filaPartida   = make(chan *Jogador, 100) // fila de matchmaking
	partidasAtivas = map[string]*Partida{}   // partidas em andamento
	partidasMu     sync.Mutex                // mutex para proteger partidasAtivas

	// inventário de boosters
	boostersMu sync.Mutex
	boosters   []*PacoteBooster

	cartasDisponiveis = map[int]string{} // catálogo de cartas do jogo

	// gerador de números aleatórios
	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Inicializa boosters e cartas
	prepararBoosters(50) // cria 50 pacotes booster
	inicializarCartas()  // inicializa catálogo de cartas

	// Inicia respondedor de ping UDP
	go iniciarRespondedorUDP(":4001")

	// Inicia servidor TCP do lobby
	ln, err := net.Listen("tcp", ":4000")
	if err != nil {
		log.Fatal("Erro ao escutar:", err)
	}
	defer ln.Close()
	log.Println("Servidor TCP do lobby ouvindo em :4000")

	// Loop de matchmaking para criar partidas
	go loopPartidas()

	// Loop principal de aceitação de conexões TCP
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Erro ao aceitar conexão:", err)
			continue
		}
		go lidarConexao(conn) // trata cada jogador em goroutine separada
	}
}

// cria o catálogo de cartas do jogo com raridades
func inicializarCartas() {
	for i := 1; i <= 20; i++ {
		raridade := "Comum"
		if i <= 5 {
			raridade = "Rara"
		} else if i <= 10 {
			raridade = "Incomum"
		}
		cartasDisponiveis[i] = fmt.Sprintf("Carta %d (%s)", i, raridade)
	}
	log.Printf("Inicializado catálogo de %d cartas\n", len(cartasDisponiveis))
}

// gera pacotes booster com cartas aleatórias
func prepararBoosters(n int) {
	boostersMu.Lock()
	defer boostersMu.Unlock()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("booster-%04d", i+1)
		cartas := []string{
			fmt.Sprintf("C%03d-R", rnd.Intn(100)), // carta rara
			fmt.Sprintf("C%03d-U", rnd.Intn(200)), // carta incomum
			fmt.Sprintf("C%03d-C", rnd.Intn(300)), // carta comum
		}
		boosters = append(boosters, &PacoteBooster{ID: id, Cartas: cartas})
	}
	log.Printf("Preparados %d boosters\n", len(boosters))
}

// inicia um servidor UDP que responde "pong" a pings
func iniciarRespondedorUDP(endereco string) {
	pc, err := net.ListenPacket("udp", endereco)
	if err != nil {
		log.Fatal("Erro ao iniciar UDP:", err)
	}
	defer pc.Close()
	log.Println("Respondedor UDP ouvindo em", endereco)
	buf := make([]byte, 1024)
	for {
		n, raddr, err := pc.ReadFrom(buf)
		if err != nil {
			log.Println("Erro ao ler UDP:", err)
			continue
		}
		payload := strings.TrimSpace(string(buf[:n]))
		_ = payload
		_, _ = pc.WriteTo([]byte("pong\n"), raddr) // responde "pong"
	}
}

// gerencia a conexão TCP de um jogador, leitura de comandos e mensagens
func lidarConexao(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	// solicita nome do jogador
	nomeLinha, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Erro ao ler nome:", err)
		return
	}
	nome := strings.TrimSpace(nomeLinha)
	if nome == "" {
		nome = "Jogador"
	}
	jogadorID := fmt.Sprintf("%d", time.Now().UnixNano()) // ID único baseado em timestamp

	// cria estrutura do jogador
	j := &Jogador{
		ID:          jogadorID,
		Nome:        nome,
		Conexao:     conn,
		Saida:       make(chan string, 10),
		EmPartida:   false,
		EnderecoUDP: "localhost:4001",
	}

	// adiciona jogador à lista global
	jogadoresMu.Lock()
	jogadores[j.ID] = j
	jogadoresMu.Unlock()

	j.enviarMensagem(fmt.Sprintf("Ping UDP: %s\n", j.EnderecoUDP))
	j.enviarMensagem("Comandos: /entrar, /sair, /jogar <idCarta>, /mao, /cartas, /fim, /booster, ou mensagens de chat\n")

	// goroutine que envia mensagens ao jogador
	go escritorJogador(j)

	// loop de leitura de mensagens do jogador
	for {
		linha, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Jogador %s desconectou: %v\n", j.Nome, err)
			removerJogador(j)
			return
		}
		linha = strings.TrimSpace(linha)
		if linha == "/" {
			continue
		}

		// comandos iniciados por "/"
		if strings.HasPrefix(linha, "/") {
			tratarComando(j, linha)
			continue
		}

		// ações em JSON
		if strings.HasPrefix(linha, "{") {
			var acao AcaoJogo
			if err := json.Unmarshal([]byte(linha), &acao); err != nil {
				j.enviarMensagem("Ação inválida (JSON incorreto)\n")
				continue
			}
			tratarAcao(j, acao)
		} else {
			// mensagem de chat normal
			transmitir(fmt.Sprintf("[%s] %s", j.Nome, linha), j)
		}
	}
}

// envia uma mensagem para o jogador sem travar caso o canal esteja cheio
func (j *Jogador) enviarMensagem(msg string) {
	select {
	case j.Saida <- msg:
	default:
	}
}

// escreve continuamente mensagens do canal para a conexão TCP
func escritorJogador(j *Jogador) {
	for msg := range j.Saida {
		_, err := j.Conexao.Write([]byte(msg + "\n"))
		if err != nil {
			log.Println("Erro ao escrever para", j.ID, err)
			return
		}
	}
}

// remove o jogador da lista global e encerra partidas ativas se necessário
func removerJogador(j *Jogador) {
	jogadoresMu.Lock()
	delete(jogadores, j.ID)
	jogadoresMu.Unlock()
	partidasMu.Lock()
	for mid, p := range partidasAtivas {
		if p.A.ID == j.ID || p.B.ID == j.ID {
			var oponente *Jogador
			if p.A.ID == j.ID {
				oponente = p.B
			} else {
				oponente = p.A
			}
			if oponente != nil {
				oponente.enviarMensagem("Oponente desconectou, partida encerrada")
				oponente.mu.Lock()
				oponente.EmPartida = false
				oponente.mu.Unlock()
			}
			delete(partidasAtivas, mid)
		}
	}
	partidasMu.Unlock()
	close(j.Saida) // fecha canal de saída do jogador
}

// exibe as cartas na mão do jogador
func mostrarMao(j *Jogador) {
	p := encontrarPartidaPorJogador(j.ID)
	if p == nil {
		j.enviarMensagem("Você não está em uma partida")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	mao := p.Mao[j.ID]
	var builder strings.Builder
	builder.WriteString("Sua mão:\n")
	for _, cid := range mao {
		builder.WriteString(fmt.Sprintf("  [%d] %s\n", cid, cartasDisponiveis[cid]))
	}
	j.enviarMensagem(builder.String())
}

// interpreta comandos de texto do jogador
func tratarComando(j *Jogador, linha string) {
	switch {
	case linha == "/entrar":
		// entra na fila de partidas
		j.mu.Lock()
		if j.EmPartida {
			j.mu.Unlock()
			j.enviarMensagem("Você já está em uma partida")
			return
		}
		j.mu.Unlock()
		select {
		case filaPartida <- j:
			j.enviarMensagem("Entrou na fila de partidas...")
		default:
			j.enviarMensagem("Fila cheia, tente mais tarde")
		}
	case linha == "/sair":
		j.enviarMensagem("Você saiu da fila (não implementado)")

	case linha == "/mao":
		mostrarMao(j)

	case linha == "/cartas":
		var builder strings.Builder
		builder.WriteString("Cartas do jogo:\n")
		for id, nome := range cartasDisponiveis {
			builder.WriteString(fmt.Sprintf("  [%d] %s\n", id, nome))
		}
		j.enviarMensagem(builder.String())

	case strings.HasPrefix(linha, "/jogar "):
		partes := strings.Split(linha, " ")
		if len(partes) < 2 {
			j.enviarMensagem("Uso: /jogar <id_carta>")
			return
		}
		cartaID, err := strconv.Atoi(partes[1])
		if err != nil {
			j.enviarMensagem("ID da carta inválido.")
			return
		}
		tratarAcao(j, AcaoJogo{Acao: "jogar_carta", CartaID: cartaID})

	case linha == "/booster":
		pacote, ok := pegarBooster()
		if !ok {
			j.enviarMensagem("Não há boosters disponíveis")
			return
		}
		j.enviarMensagem(fmt.Sprintf("Você abriu booster %s -> cartas: %v", pacote.ID, pacote.Cartas))

	case strings.HasPrefix(linha, "/ping"):
		j.enviarMensagem(fmt.Sprintf("Hora do servidor: %s", time.Now().Format(time.RFC3339)))

	default:
		j.enviarMensagem("Comando desconhecido")
	}
}

// realiza o matchmaking entre jogadores na fila
func loopPartidas() {
	for {
		a := <-filaPartida
		select {
		case b := <-filaPartida:
			a.mu.Lock()
			if a.EmPartida {
				a.mu.Unlock()
				go func() { filaPartida <- b }()
				continue
			}
			a.EmPartida = true
			a.mu.Unlock()

			b.mu.Lock()
			if b.EmPartida {
				b.mu.Unlock()
				a.mu.Lock()
				a.EmPartida = false
				a.mu.Unlock()
				go func() { filaPartida <- a }()
				continue
			}
			b.EmPartida = true
			b.mu.Unlock()

			criarPartida(a, b)
		case <-time.After(30 * time.Second):
			select {
			case filaPartida <- a:
			default:
			}
		}
	}
}

// retorna uma mão aleatória de cartas do jogador
func gerarMaoAleatoria(qtd int) []int {
	mao := make([]int, 0, qtd)
	ids := make([]int, 0, len(cartasDisponiveis))
	for id := range cartasDisponiveis {
		ids = append(ids, id)
	}

	for i := 0; i < qtd; i++ {
		idx := rnd.Intn(len(ids))
		mao = append(mao, ids[idx])
		// remover para não repetir cartas na mesma mão
		ids = append(ids[:idx], ids[idx+1:]...)
		if len(ids) == 0 {
			break
		}
	}
	return mao
}

// envia sinais periódicos para os jogadores da partida
func rodarPartida(p *Partida) {
	for {
		time.Sleep(30 * time.Second)
		partidasMu.Lock()
		_, ok := partidasAtivas[p.ID]
		partidasMu.Unlock()
		if !ok {
			return
		}
		p.A.enviarMensagem(fmt.Sprintf("Sinal da partida %s", p.ID))
		p.B.enviarMensagem(fmt.Sprintf("Sinal da partida %s", p.ID))
	}
}

// envia mensagem para todos jogadores fora de partidas
func transmitir(msg string, origem *Jogador) {
	jogadoresMu.Lock()
	defer jogadoresMu.Unlock()
	for _, j := range jogadores {
		j.mu.Lock()
		emPartida := j.EmPartida
		j.mu.Unlock()
		if !emPartida && j.ID != origem.ID {
			j.enviarMensagem(msg)
		}
	}
}

// inicializa uma nova partida entre dois jogadores
func criarPartida(a, b *Jogador) {
	idPartida := fmt.Sprintf("partida-%d", time.Now().UnixNano())
	p := &Partida{
		ID:     idPartida,
		A:      a,
		B:      b,
		Criada: time.Now(),
		Turno:  a.ID,
		Mao: map[string][]int{
			a.ID: gerarMaoAleatoria(5),
			b.ID: gerarMaoAleatoria(5),
		},
		Vida: map[string]int{
			a.ID: 100,
			b.ID: 100,
		},
	}

	partidasMu.Lock()
	partidasAtivas[idPartida] = p
	partidasMu.Unlock()

	a.enviarMensagem(fmt.Sprintf("\n============================\nVocê foi pareado com %s!\nID da partida: %s\nVida inicial: 100\n============================", b.Nome, idPartida))
	b.enviarMensagem(fmt.Sprintf("\n============================\nVocê foi pareado com %s!\nID da partida: %s\nVida inicial: 100\n============================", a.Nome, idPartida))

	go rodarPartida(p)
}

// calcula o dano de uma carta com base em sua raridade
func danoCarta(cartaID int) int {
	nome := cartasDisponiveis[cartaID]
	if strings.Contains(nome, "Rara") {
		return 30
	} else if strings.Contains(nome, "Incomum") {
		return 20
	}
	return 10 // Comum
}

// processa ações do jogador dentro de uma partida
func tratarAcao(j *Jogador, acao AcaoJogo) {
	switch acao.Acao {
	case "jogar_carta":
		p := encontrarPartidaPorJogador(j.ID)
		if p == nil {
			j.enviarMensagem("Você não está em uma partida")
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.Turno != j.ID {
			j.enviarMensagem("Não é sua vez")
			return
		}

		mao := p.Mao[j.ID]
		pos := -1
		for i, cid := range mao {
			if cid == acao.CartaID {
				pos = i
				break
			}
		}
		if pos == -1 {
			j.enviarMensagem("Carta não encontrada na mão")
			return
		}

		// remove carta da mão
		mao = append(mao[:pos], mao[pos+1:]...)
		p.Mao[j.ID] = mao

		// determina o oponente
		var oponenteID string
		if p.A.ID == j.ID {
			oponenteID = p.B.ID
		} else {
			oponenteID = p.A.ID
		}

		// calcula dano e aplica
		dano := danoCarta(acao.CartaID)
		p.Vida[oponenteID] -= dano
		if p.Vida[oponenteID] < 0 {
			p.Vida[oponenteID] = 0
		}

		nomeCarta := cartasDisponiveis[acao.CartaID]
		msg := fmt.Sprintf("\n%s jogou a carta [%d] %s causando %d de dano!\nVida de %s: %d | Vida de %s: %d\n",
			j.Nome, acao.CartaID, nomeCarta, dano,
			j.Nome, p.Vida[j.ID], "Oponente", p.Vida[oponenteID],
		)
		p.A.enviarMensagem(msg)
		p.B.enviarMensagem(msg)

		// verifica vitória
		if p.Vida[oponenteID] <= 0 {
			p.A.enviarMensagem(fmt.Sprintf("\n============================\n%s venceu a partida!\n============================", j.Nome))
			p.B.enviarMensagem(fmt.Sprintf("\n============================\n%s venceu a partida!\n============================", j.Nome))
			p.A.mu.Lock()
			p.A.EmPartida = false
			p.A.mu.Unlock()
			p.B.mu.Lock()
			p.B.EmPartida = false
			p.B.mu.Unlock()

			partidasMu.Lock()
			delete(partidasAtivas, p.ID)
			partidasMu.Unlock()
		}

	case "fim_turno":
		p := encontrarPartidaPorJogador(j.ID)
		if p == nil {
			j.enviarMensagem("Você não está em uma partida")
			return
		}
		p.mu.Lock()
		if p.Turno == p.A.ID {
			p.Turno = p.B.ID
		} else {
			p.Turno = p.A.ID
		}
		p.A.enviarMensagem(fmt.Sprintf("\n============================\nVez trocada! Agora: %s\n============================", p.Turno))
		p.B.enviarMensagem(fmt.Sprintf("\n============================\nVez trocada! Agora: %s\n============================", p.Turno))
		p.mu.Unlock()
	}
}

// retorna a partida em que o jogador está
func encontrarPartidaPorJogador(jogadorID string) *Partida {
	partidasMu.Lock()
	defer partidasMu.Unlock()
	for _, p := range partidasAtivas {
		if p.A.ID == jogadorID || p.B.ID == jogadorID {
			return p
		}
	}
	return nil
}

// retorna um pacote booster disponível ou false se não houver
func pegarBooster() (*PacoteBooster, bool) {
	boostersMu.Lock()
	defer boostersMu.Unlock()
	if len(boosters) == 0 {
		return nil, false
	}
	idx := len(boosters) - 1
	pacote := boosters[idx]
	boosters = boosters[:idx] // remove do inventário
	return pacote, true
}
