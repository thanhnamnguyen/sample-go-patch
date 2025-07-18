generate:
	protoc --go_out=paths=source_relative:. patch/go.proto