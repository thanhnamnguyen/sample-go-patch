generate:
	protoc --go_out=paths=source_relative:. patch/go.proto

install:
	go install ./cmd/protoc-gen-go-patch

test:
	protoc --go-patch_out=plugin=go,paths=source_relative:. \
	--go-patch_out=plugin=go-grpc,paths=source_relative:.  \
	--go-vtproto_out=paths=source_relative:. \
    --go-vtproto_opt=features=marshal+unmarshal+size \
	./tests/nullable_test.proto