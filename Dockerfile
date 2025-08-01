FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o myapp

FROM scratch

COPY --from=build /app/myapp .

EXPOSE 8001

ENTRYPOINT [ "/myapp" ]