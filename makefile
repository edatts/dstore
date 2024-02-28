build:
	go build -o ./bin/dstore

run: build
	./bin/dstore

test: build
	go test ./... -v