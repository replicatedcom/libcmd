.PHONY: clean godep deps run test vet build all

clean:
	rm -rf _vendor
	rm -f ./bin/libcmd

godep:
	go get github.com/tools/godep

deps: godep
	godep go install

run: build
	sudo ./bin/libcmd -cmd=raw echo hi

test: build
	cd tests && ./test.sh

vet:
	cd tests && ./vet.sh

build:
	mkdir -p bin
	godep go build -o bin/libcmd ./run

all: build vet test
