# Usa imagem base do Go
FROM golang:1.22


# Cria diretório de trabalho
WORKDIR /app

# Copia os arquivos go.mod e go.sum primeiro 
COPY go.mod ./
RUN go mod download

# Copia o restante do código
COPY . .

# Compila o servidor
RUN go build -o server ./cmd/server/main.go

# Expõe a porta do lobby
EXPOSE 4000
EXPOSE 5000/udp

# Comando 
CMD ["./server"]
