package ftp

import (
	"net"
	"time"

	ftpServer "github.com/jlaffaye/ftp"
)

type Config struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

func Connect(config Config) (*ftpServer.ServerConn, error) {
	address := net.JoinHostPort(config.Host, config.Port)
	connection, err := ftpServer.Dial(address, ftpServer.DialWithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	if err := connection.Login(config.User, config.Password); err != nil {
		_ = connection.Quit()
		return nil, err
	}

	return connection, nil
}
