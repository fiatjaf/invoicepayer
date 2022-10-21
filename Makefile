invoicepayer: $(shell find . -name "*.go") $(shell find . -name "*.html")
	CC=$$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./invoicepayer
