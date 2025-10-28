
.PHONY: clone_helm compile_helm build run

all: build run

clone_helm:
	git clone https://github.com/helm/helm

compile_helm:
	sed -i 's/LDFLAGS\s*:=.*/LDFLAGS := /' helm/Makefile
	sed -i 's/GOFLAGS\s*:=.*/GOFLAGS := -gcflags="all=-N -l"/' helm/Makefile
	cd helm && make

build:
	go build

run:
	go run . line
