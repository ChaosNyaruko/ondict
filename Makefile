.PHONY: run debug install 
word=doctor

# one-shot run
run: 
	go run . -q=$(word) 

serve:
	go run . -serve

auto:
	go run . -q=$(word) -remote=auto

debug:
	go run . -d=true -v 

install:
	go install .
