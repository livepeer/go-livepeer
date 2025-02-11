package starter

import (
	"github.com/golang/glog"
	lpmon "github.com/livepeer/go-livepeer/monitor"
)

func startKafkaProducer(cfg LivepeerConfig) error {
	if *cfg.KafkaBootstrapServers == "" || *cfg.KafkaUsername == "" || *cfg.KafkaPassword == "" || *cfg.KafkaGatewayTopic == "" {
		glog.Warning("not starting Kafka producer as producer config values aren't present")
		return nil
	}

	var gatewayHost = ""
	if cfg.GatewayHost != nil {
		gatewayHost = *cfg.GatewayHost
	}

	return lpmon.InitKafkaProducer(
		*cfg.KafkaBootstrapServers,
		*cfg.KafkaUsername,
		*cfg.KafkaPassword,
		*cfg.KafkaGatewayTopic,
		gatewayHost,
	)
}
