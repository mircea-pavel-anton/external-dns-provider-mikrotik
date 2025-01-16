package configuration

import (
	"sigs.k8s.io/external-dns/endpoint"
	"time"

	"github.com/caarlos0/env/v11"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ServerHost           string        `env:"SERVER_HOST" envDefault:"localhost"`
	ServerPort           int           `env:"SERVER_PORT" envDefault:"8888"`
	ServerReadTimeout    time.Duration `env:"SERVER_READ_TIMEOUT"`
	ServerWriteTimeout   time.Duration `env:"SERVER_WRITE_TIMEOUT"`
	DomainFilter         []string      `env:"DOMAIN_FILTER" envDefault:""`
	ExcludeDomains       []string      `env:"EXCLUDE_DOMAIN_FILTER" envDefault:""`
	RegexDomainFilter    string        `env:"REGEXP_DOMAIN_FILTER" envDefault:""`
	RegexDomainExclusion string        `env:"REGEXP_DOMAIN_FILTER_EXCLUSION" envDefault:""`
	DefaultTTl           endpoint.TTL  `env:"DEFAULT_TTL" envDefault:"300"`
}

func Init() Config {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("error reading configuration from environment: %v", err)
	}
	return cfg
}
