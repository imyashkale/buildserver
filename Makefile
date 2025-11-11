run:
	go run cmd/main.go

build:
	go build -o buildserver cmd/main.go

run-build:
	./buildserver

clean:
	rm -f buildserver