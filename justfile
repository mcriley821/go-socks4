[private]
default:
  @just --list --justfile {{justfile()}}

build:
  @mkdir -p bin
  cd src && go build -o ../bin/ ./main.go

test *flags:
  cd src && go test --coverprofile ../coverage.prof {{flags}} ./...
  cd src && go tool cover -html ../coverage.prof -o ../coverage.html

fmt:
  cd src && go fmt ./...

vet:
  cd src && go vet ./... && echo && staticcheck ./...

