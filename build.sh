#!/bin/bash
mkdir -p bin
GOOS=darwin GOARCH=amd64 go build -o bin/conductor-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o bin/conductor-darwin-arm64 main.go
GOOS=linux GOARCH=amd64 go build -o bin/conductor-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -o bin/conductor-linux-arm64 main.go
GOOS=windows GOARCH=amd64 go build -o bin/conductor-windows-amd64.exe main.go
GOOS=windows GOARCH=arm64 go build -o bin/conductor-windows-arm64.exe main.go
