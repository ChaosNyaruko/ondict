.PHONY: run debug install serve mdx interactive local auto
word=doctor
version := $(shell git describe --tags)
commit := $(shell git rev-parse HEAD)

# one-shot run
run: 
	go run . -q=$(word) 

serve:
	go run . -serve -f=md -e=mdx -listen=0.0.0.0:1345 
	# go run . -serve -f=md -e=mdx -listen=127.0.0.1:1345 -v
	#
serve-v:
	go run . -serve -f=md -e=mdx -listen=0.0.0.0:1345 -v
	# go run . -serve -f=md -e=mdx -listen=127.0.0.1:1345 -v
	#
query-online:
	go run . -f=md -e=online -q=$(word)

local:
	go run . -f=md -e=mdx -v -q=apple

auto:
	go run . -q=$(word) -remote=auto -e=mdx -f=md

debug:
	go run . -d=true -v

mdx:
	go run . -e=mdx -q=$(word) -v

interactive:
	go run . -i -e=mdx -f=md

build:
	./build.sh

install:
	go install -ldflags "-X main.Version=$(version) -X main.Commit=$(commit)" .

localtest:
	FULLTEST=1 go test -v ./...

test:
	go test ./... -coverprofile=cover.out  -v
	go tool cover -func cover.out | tail -1
	go tool cover -html=cover.out -o cover.html

play:
	@echo $$$$
	@echo $$$$
	@echo $$$$
	@echo $$$$

docker:
	 docker run --rm --name ondict-app --publish 1346:1345 --mount type=bind,source=/Users/bytedance/.config/ondict,target=/root/.config/ondict  ondict
