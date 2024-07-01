#!/bin/bash
mkdir -p bin
GOOS=darwin GOARCH=amd64 go build -o bin/ocl-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o bin/ocl-darwin-arm64 main.go
GOOS=linux GOARCH=amd64 go build -o bin/ocl-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -o bin/ocl-linux-arm64 main.go
GOOS=windows GOARCH=amd64 go build -o bin/ocl-windows-amd64.exe main.go
GOOS=windows GOARCH=arm64 go build -o bin/ocl-windows-arm64.exe main.go
