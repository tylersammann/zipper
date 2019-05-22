
.PHONY: generate, run

generate-static:
	go run -tags generate generate.go

run: generate-static
	go run main.go