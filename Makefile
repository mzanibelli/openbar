all: lint test install

lint:
	golangci-lint run ./...

test:
	go test -race ./...

install:
	go install cmd/openbar.go

reload:
	swaymsg reload
