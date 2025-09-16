# Lobby Game Server

```markdown


## Descrição
Este projeto implementa um servidor de lobby para um jogo de cartas, permitindo:

- Conexão de múltiplos jogadores via TCP  
- Comunicação de ping via UDP  
- Sistema de matchmaking automático  
- Partidas entre dois jogadores com cartas e vida  
- Boosters de cartas aleatórias  
- Testes de carga com múltiplos clientes simultâneos  

---

## Estrutura do Projeto
```

```

.
├── cmd
│   ├── client
│   │   └── main.go       # Cliente TCP para interagir com o lobby
│   ├── server
│   │   └── main.go       # Código do servidor
│   └── test
│       └── load\_tester.go # Código do load tester
├── Dockerfile             # Imagem Docker para servidor e load tester
├── docker-compose.yml     # Orquestração dos serviços
└── go.mod                 # Dependências Go

```

---

## Requisitos

- Go 1.22+  
- Docker & Docker Compose (opcional, para execução em containers)

---

## Rodando Localmente

### 1. Servidor
```bash
go run cmd/server/main.go
````

O servidor TCP ficará escutando na porta `4000` e UDP em `5000`.

### 2. Client

```bash
go run cmd/client/main.go
```

Após conectar, o jogador pode usar comandos:

* `/entrar` → entra na fila de partidas
* `/sair` → sai da fila (não implementado)
* `/mao` → mostra cartas na mão
* `/cartas` → lista todas as cartas do jogo
* `/jogar <idCarta>` → joga uma carta
* `/fim` → termina o turno
* `/booster` → abre um pacote booster
* `/ping` → mostra latência da rede do usuário
* Mensagens sem `/` → chat global

#### Exemplo de sessão no client

```text
Bem-vindo, Jogador: Alice
Ping UDP: localhost:4001
Comandos: /entrar, /sair, /jogar <idCarta>, /mao, /cartas, /fim, /booster

> /cartas
Cartas do jogo:
  [1] Carta 1 (Rara)
  [2] Carta 2 (Rara)
  ...
  [20] Carta 20 (Comum)

> /booster
Você abriu booster booster-0001 -> cartas: [C034-R C121-U C215-C]

> /entrar
Entrou na fila de partidas...

============================
Você foi pareado com Bob!
ID da partida: partida-169468
Vida inicial: 100
============================

> /mao
Sua mão:
  [3] Carta 3 (Comum)
  [7] Carta 7 (Incomum)
  [12] Carta 12 (Comum)
  [15] Carta 15 (Rara)
  [20] Carta 20 (Comum)

> /jogar 15
Alice jogou a carta [15] Carta 15 (Rara) causando 30 de dano!
Vida de Alice: 100 | Vida de Oponente: 70
============================
Vez trocada! Alice passou a vez
============================
```

---

## Rodando via Docker

### 1. Build da imagem

```bash
docker-compose build
```

### 2. Rodando o servidor + load tester

```bash
docker-compose up
```

O `lobby` ficará disponível em `localhost:4000` (TCP) e `localhost:5000` (UDP). O `tester` iniciará automaticamente simulando múltiplos clientes.

---

## Load Tester

Você pode rodar o teste manualmente:

```bash
go run cmd/test/load_tester.go -clients 200 -duration 60s -addr localhost:4000
```

Parâmetros:

* `-clients` → número de clientes simultâneos
* `-duration` → duração do teste (`s`, `m`, `h`)
* `-addr` → endereço do servidor

### Saída típica do load tester

```text
=== Resultado do Load Test ===
Clientes requisitados: 200
Conexões bem-sucedidas: 198
Conexões falhas: 2
Ações enviadas: 5123 (erros: 8)
Vitórias detectadas: 45
Latências registradas: 5115
avg: 150ms, p50: 120ms, p90: 250ms, p99: 400ms
```

---

## Observações

* O servidor utiliza goroutines para cada jogador, garantindo alta simultaneidade.
* Boosters e cartas são gerados aleatoriamente a cada inicialização.
* UDP é usado apenas para responder pings; toda lógica de partidas é via TCP.
* Partidas terminam quando a vida de um jogador chega a 0.

```

  
