package starter

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"
	lpmon "github.com/livepeer/go-livepeer/monitor"
)

func startEventPublisher(cfg LivepeerConfig) error {
	sinkList := splitList(*cfg.EventSinkURIs)
	if len(sinkList) == 0 {
		legacyKafkaURI, err := buildLegacyKafkaSink(cfg)
		if err != nil {
			return err
		}
		if legacyKafkaURI != "" {
			sinkList = append(sinkList, legacyKafkaURI)
		}
	}

	if len(sinkList) == 0 {
		glog.Warning("event publisher not started: no sinks configured")
		return nil
	}

	headers, err := parseHeaderList(*cfg.EventSinkHeaders)
	if err != nil {
		return err
	}

	queueDepth := valueOrDefaultInt(cfg.EventSinkQueueDepth, 100)
	batchSize := valueOrDefaultInt(cfg.EventSinkBatchSize, 100)
	flushInterval := valueOrDefaultDuration(cfg.EventSinkFlushInterval, time.Second)

	publisherCfg := lpmon.PublisherConfig{
		GatewayAddress: valueOrDefaultString(cfg.GatewayHost, ""),
		QueueSize:      queueDepth,
		BatchSize:      batchSize,
		FlushInterval:  flushInterval,
		SinkURLs:       sinkList,
		Headers:        headers,
	}

	if err := lpmon.InitEventPublisher(publisherCfg); err != nil {
		return fmt.Errorf("init event publisher: %w", err)
	}

	return nil
}

func splitList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', ';':
			return true
		default:
			return false
		}
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func parseHeaderList(raw string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, entry := range splitList(raw) {
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid header %q, expected Key=Value", entry)
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key == "" {
			return nil, fmt.Errorf("invalid header %q: empty key", entry)
		}
		headers[key] = value
	}
	return headers, nil
}

func buildLegacyKafkaSink(cfg LivepeerConfig) (string, error) {
	if cfg.KafkaBootstrapServers == nil || cfg.KafkaGatewayTopic == nil {
		return "", nil
	}
	brokers := strings.TrimSpace(*cfg.KafkaBootstrapServers)
	topic := strings.TrimSpace(*cfg.KafkaGatewayTopic)
	if brokers == "" || topic == "" {
		return "", nil
	}
	var user, password string
	if cfg.KafkaUsername != nil {
		user = strings.TrimSpace(*cfg.KafkaUsername)
	}
	if cfg.KafkaPassword != nil {
		password = strings.TrimSpace(*cfg.KafkaPassword)
	}

	brokerList := strings.Split(brokers, ",")
	host := strings.TrimSpace(brokerList[0])
	if host == "" {
		return "", fmt.Errorf("invalid Kafka bootstrap server string %q", brokers)
	}

	u := url.URL{Scheme: "kafka", Host: host}
	if user != "" {
		if password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
	}
	q := u.Query()
	q.Set("topic", topic)
	q.Set("brokers", brokers)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func valueOrDefaultString(ptr *string, def string) string {
	if ptr == nil {
		return def
	}
	if v := strings.TrimSpace(*ptr); v != "" {
		return v
	}
	return def
}

func valueOrDefaultInt(ptr *int, def int) int {
	if ptr != nil && *ptr > 0 {
		return *ptr
	}
	return def
}

func valueOrDefaultDuration(ptr *time.Duration, def time.Duration) time.Duration {
	if ptr != nil && *ptr > 0 {
		return *ptr
	}
	return def
}
