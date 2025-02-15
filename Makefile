.PHONY: proto

all: proto

proto: gen/proto gen/proto/manifest.pb.go

gen/proto:
	mkdir -p gen/proto

gen/proto/%.pb.go: proto/%.proto
	protoc -I=. --go_out=gen --go-grpc_out=gen \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$<
