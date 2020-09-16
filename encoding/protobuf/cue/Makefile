default:
	mkdir -p ./_output
	protoc --go_out=./_output cue.proto
	cp ./_output/cuelang.org/go/encoding/protobuf/cue/cue.pb.go .
	rm -rf ./_output

	go test ./...

clean:
	rm -rf ./_output
