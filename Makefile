build: *.go
	go build -o qq *.go

run: build
	./qq
