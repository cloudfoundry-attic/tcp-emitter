package config

import (
	"io/ioutil"

	"github.com/cloudfoundry-incubator/candiedyaml"
)

type RoutingAPIConfig struct {
	URI          string `yaml:"uri"`
	Port         int    `yaml:"port"`
	AuthDisabled bool   `yaml:"auth_disabled"`
}

type OAuthConfig struct {
	UaaInternalHostname    string `yaml:"uaa_internal_hostname"`
	UaaTLSPort             int    `yaml:"uaa_tls_port"`
	SkipUaaTLSVerification bool   `yaml:"skip_uaa_tls_verification"`
	ClientName             string `yaml:"client_name"`
	ClientSecret           string `yaml:"client_secret"`
}

type Config struct {
	OAuth      OAuthConfig      `yaml:"oauth"`
	RoutingAPI RoutingAPIConfig `yaml:"routing_api"`
}

func New(path string) (*Config, error) {
	c := &Config{}
	err := c.initConfigFromFile(path)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) initConfigFromFile(path string) error {
	var e error

	b, e := ioutil.ReadFile(path)
	if e != nil {
		return e
	}

	return candiedyaml.Unmarshal(b, &c)
}
