FLAGS=GOFLAGS="-buildvcs=false"

.PHONY: build
build: lint
	$(FLAGS) go build -o _build/metallb-converter .

.PHONY: run
run:
	_build/metallb-converter

.PHONY: run-examples
run-examples:
	rm -f _output/*
	_build/metallb-converter -input-dir _examples/ -output-dir _output/

.PHONY: lint
lint:
	$(FLAGS) golangci-lint run

.PHONY: test
test:
	$(FLAGS)  go test ./... -v -race -cover

.PHONY: coverprofile
coverprofile:
	go test -coverprofile=_output/coverprofile.out ./...
	go tool cover -html=_output/coverprofile.out
