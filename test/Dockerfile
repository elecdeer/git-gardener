# ホスト環境の git 設定に依存しない Linux 上でテストを実行する
FROM golang:1.27

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["go", "test", "./..."]
