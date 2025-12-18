build:
	go vet ./...
	go fmt ./...
	go mod tidy
	CGO_ENABLED=0 go build -o ./bin/example-app main.go

image:
	docker buildx build . -t ghcr.io/jjsiv/autopay-exercise/example-app:master --push

