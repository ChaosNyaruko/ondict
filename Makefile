.PHONY: run debug install 
run:
	go run . -q=$(word) 

test:
	go run . -q=doctor -d=true 

easy:
	go run . -q=doctor -e=true -d=true 

debug:
	go run . -q=doctor -d=true -v

install:
	go install .
