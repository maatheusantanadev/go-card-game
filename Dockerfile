# Usa imagem base do Go
FROM golang:1.22.4-slim

# Cria diretório de trabalho
WORKDIR /app

# Copia os arquivos go.mod e go.sum primeiro 
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código
COPY . .

# Compila o servidor
RUN go build -o server main.go

# Expõe a porta do lobby
EXPOSE 4000

# Comando 
CMD ["./server"]
