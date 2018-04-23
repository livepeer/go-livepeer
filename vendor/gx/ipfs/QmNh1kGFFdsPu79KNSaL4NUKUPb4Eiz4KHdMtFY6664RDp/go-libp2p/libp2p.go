package libp2p

import (
	"context"
	"crypto/rand"
	"fmt"

	yamux "gx/ipfs/QmNWCEvi7bPRcvqAV8AKLGVNoQdArWi7NJayka2SM4XtRe/go-smux-yamux"
	bhost "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/host/basic"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	swarm "gx/ipfs/QmSwZMWwFZSUpe5muU2xgTUwppH24KfMwdPXiwbEp2c6G5/go-libp2p-swarm"
	msmux "gx/ipfs/QmVniQJkdzLZaZwzwMdd3dJTvWiJ1DQEkreVy6hs6h7Vk5/go-smux-multistream"
	transport "gx/ipfs/QmVxtCwKFMmwcjhQXsGj6m4JAW7nGb9hRoErH9jpgqcLxA/go-libp2p-transport"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	mux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	pnet "gx/ipfs/QmZPrWxuM8GHr4cGKbyF5CCT11sFUP9hgqpeUHALvx2nUr/go-libp2p-interface-pnet"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	mplex "gx/ipfs/Qmc14vuKyGqX27RvBhekYytxSFJpaEgQVuVJgKSm69MEix/go-smux-multiplex"
	metrics "gx/ipfs/QmdeBtQGXjSt7cb97nx9JyLHHv5va2LyEAue7Q5tDFzpLy/go-libp2p-metrics"
)

// Config describes a set of settings for a libp2p node
type Config struct {
	Transports   []transport.Transport
	Muxer        mux.Transport
	ListenAddrs  []ma.Multiaddr
	PeerKey      crypto.PrivKey
	Peerstore    pstore.Peerstore
	Protector    pnet.Protector
	Reporter     metrics.Reporter
	DisableSecio bool
}

type Option func(cfg *Config) error

func Transports(tpts ...transport.Transport) Option {
	return func(cfg *Config) error {
		cfg.Transports = append(cfg.Transports, tpts...)
		return nil
	}
}

func ListenAddrStrings(s ...string) Option {
	return func(cfg *Config) error {
		for _, addrstr := range s {
			a, err := ma.NewMultiaddr(addrstr)
			if err != nil {
				return err
			}
			cfg.ListenAddrs = append(cfg.ListenAddrs, a)
		}
		return nil
	}
}

func ListenAddrs(addrs ...ma.Multiaddr) Option {
	return func(cfg *Config) error {
		cfg.ListenAddrs = append(cfg.ListenAddrs, addrs...)
		return nil
	}
}

type transportEncOpt int

const (
	EncPlaintext = transportEncOpt(0)
	EncSecio     = transportEncOpt(1)
)

func TransportEncryption(tenc ...transportEncOpt) Option {
	return func(cfg *Config) error {
		if len(tenc) != 1 {
			return fmt.Errorf("can only specify a single transport encryption option right now")
		}

		// TODO: actually make this pluggable, otherwise tls will get tricky
		switch tenc[0] {
		case EncPlaintext:
			cfg.DisableSecio = true
		case EncSecio:
			// noop
		default:
			return fmt.Errorf("unrecognized transport encryption option: %d", tenc[0])
		}
		return nil
	}
}

func NoEncryption() Option {
	return TransportEncryption(EncPlaintext)
}

func Muxer(m mux.Transport) Option {
	return func(cfg *Config) error {
		if cfg.Muxer != nil {
			return fmt.Errorf("cannot specify multiple muxer options")
		}

		cfg.Muxer = m
		return nil
	}
}

func Peerstore(ps pstore.Peerstore) Option {
	return func(cfg *Config) error {
		if cfg.Peerstore != nil {
			return fmt.Errorf("cannot specify multiple peerstore options")
		}

		cfg.Peerstore = ps
		return nil
	}
}

func PrivateNetwork(prot pnet.Protector) Option {
	return func(cfg *Config) error {
		if cfg.Protector != nil {
			return fmt.Errorf("cannot specify multiple private network options")
		}

		cfg.Protector = prot
		return nil
	}
}

func BandwidthReporter(rep metrics.Reporter) Option {
	return func(cfg *Config) error {
		if cfg.Reporter != nil {
			return fmt.Errorf("cannot specify multiple bandwidth reporter options")
		}

		cfg.Reporter = rep
		return nil
	}
}

func Identity(sk crypto.PrivKey) Option {
	return func(cfg *Config) error {
		if cfg.PeerKey != nil {
			return fmt.Errorf("cannot specify multiple identities")
		}

		cfg.PeerKey = sk
		return nil
	}
}

func New(ctx context.Context, opts ...Option) (host.Host, error) {
	var cfg Config
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	return newWithCfg(ctx, &cfg)
}

func newWithCfg(ctx context.Context, cfg *Config) (host.Host, error) {
	// If no key was given, generate a random 2048 bit RSA key
	if cfg.PeerKey == nil {
		priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {
			return nil, err
		}
		cfg.PeerKey = priv
	}

	// Obtain Peer ID from public key
	pid, err := peer.IDFromPublicKey(cfg.PeerKey.GetPublic())
	if err != nil {
		return nil, err
	}

	// Create a new blank peerstore if none was passed in
	ps := cfg.Peerstore
	if ps == nil {
		ps = pstore.NewPeerstore()
	}

	// Set default muxer if none was passed in
	muxer := cfg.Muxer
	if muxer == nil {
		muxer = DefaultMuxer()
	}

	// If secio is disabled, don't add our private key to the peerstore
	if !cfg.DisableSecio {
		ps.AddPrivKey(pid, cfg.PeerKey)
		ps.AddPubKey(pid, cfg.PeerKey.GetPublic())
	}

	swrm, err := swarm.NewSwarmWithProtector(ctx, cfg.ListenAddrs, pid, ps, cfg.Protector, muxer, cfg.Reporter)
	if err != nil {
		return nil, err
	}

	netw := (*swarm.Network)(swrm)

	return bhost.New(netw), nil
}

func DefaultMuxer() mux.Transport {
	// Set up stream multiplexer
	tpt := msmux.NewBlankTransport()

	// By default, support yamux and multiplex
	tpt.AddTransport("/yamux/1.0.0", yamux.DefaultTransport)
	tpt.AddTransport("/mplex/6.3.0", mplex.DefaultTransport)

	return tpt
}

func Defaults(cfg *Config) error {
	// Create a multiaddress that listens on a random port on all interfaces
	addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return err
	}

	cfg.ListenAddrs = []ma.Multiaddr{addr}
	cfg.Peerstore = pstore.NewPeerstore()
	cfg.Muxer = DefaultMuxer()
	return nil
}
