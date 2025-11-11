
.PHONY: clone_helm compile_helm build run test_values_query test_helpers_query test_template_query test_rendered_query test_all_queries

all: build run

clone_helm:
	git clone https://github.com/helm/helm

compile_helm:
	sed -i 's/LDFLAGS\s*:=.*/LDFLAGS := /' helm/Makefile
	sed -i 's/GOFLAGS\s*:=.*/GOFLAGS := -gcflags="all=-N -l"/' helm/Makefile
	cd helm && make

build:
	go build

run: test_all_queries

test_values_query:
	go run . --mode model --values image.tag --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_helpers_query:
	go run . --mode model --helper-file test.serviceAccountName --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_template_query:
	go run . --mode model --template-file test/templates/deployment.yaml:42 --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_rendered_query:
	go run . --mode model --rendered-file test/templates/deployment.yaml:32 --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_all_queries:
	go run . --mode model \
		--rendered-file test/templates/deployment.yaml:32 \
		--template-file test/templates/deployment.yaml:42 \
		--helper-file test.serviceAccountName \
		--values image.tag \
		--chart test \
		--extra-command-args '--show-only templates/deployment.yaml'
