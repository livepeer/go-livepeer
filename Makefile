all: server/lp_rpc.pb.go

server/lp_rpc.pb.go: server/lp_rpc.proto
	protoc -I=. --go_out=plugins=grpc:. $^
