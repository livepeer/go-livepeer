FILES=basic_broadcaster.go basic_in_stream.go basic_network.go basic_notifiee.go \
	basic_out_stream.go basic_relayer.go basic_reporter.go basic_subscriber.go \
	data.go network_node.go peerCache.go

TESTS=basic_network integration libp2p_playground network_node

$(foreach t, $(TESTS), $(t)_test): %: $(FILES) utils_test.go
	go test $*.go $^ -logtostderr=true
