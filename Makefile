.PHONY: build install run test vet clean

build:
	go build -trimpath -o thisyou ./cmd/thisyou

install:
	go install ./cmd/thisyou

run:
	go run ./cmd/thisyou

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f thisyou
