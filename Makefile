.PHONY: run debug install 
word=doctor

# one-shot run
run: 
	go run . -q=$(word) -color

serve:
	go run . -serve

auto:
	go run . -q=$(word) -remote=auto

debug:
	go run . -d=true -v -color

install:
	go install .
