.PHONY: run debug install serve mdx interactive local auto
word=doctor

# one-shot run
run: 
	go run . -q=$(word) 

serve:
	go run . -serve -f=md -e=mdx -listen=0.0.0.0:1345 -v
	# go run . -serve -f=md -e=mdx -listen=127.0.0.1:1345 -v

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

install:
	go install .

localtest:
	FULLTEST=1 go test -v ./...

test:
	go test ./... -coverprofile=cover.out  -v
	go tool cover -func cover.out | tail -1
	go tool cover -html=cover.out -o cover.html
