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

	var broadcasterEthAddress = ""
	if cfg.EthAcctAddr != nil {
		broadcasterEthAddress = *cfg.EthAcctAddr
	}

	return lpmon.InitKafkaProducer(
		*cfg.KafkaBootstrapServers,
		*cfg.KafkaUsername,
		*cfg.KafkaPassword,
		*cfg.KafkaGatewayTopic,
		broadcasterEthAddress,
	)
}
