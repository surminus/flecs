run:
	@go run *.go

deploy:
	@go run *.go deploy

install:
	@go build
	@mv flecs ~/bin/
