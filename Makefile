.PHONY: run debug install 
run:
	go run . -q=$(word) 2>/dev/null

test:
	go run . -q=doctor -d=true 2>/dev/null

easy:
	go run . -q=doctor -e=true -d=true 2>/dev/null

debug:
	go run . -q=doctor -d=true 2>&1

install:
	go install .
