.PHONY: build check test vet fmt run size docker-build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/preprint-af ./cmd/preprint-af

test:
	go test ./...

vet:
	go vet ./...

check: vet test build

fmt:
	gofmt -w .

run: build
	cd .. && ./go/bin/preprint-af

size: build
	@ls -lh bin/preprint-af

docker-build:
	docker build -f go/Dockerfile -t preprint-af-go:latest ..
