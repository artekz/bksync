package ftp

import (
	"fmt"
	"time"

	ftpServer "github.com/jlaffaye/ftp"
	"github.com/spf13/viper"
)

func Connect() (*ftpServer.ServerConn, error) {
	ftpUrl := fmt.Sprintf("%s:%s", viper.GetString("ftp.host"), viper.GetString("ftp.port"))

	connection, err := ftpServer.Dial(ftpUrl, ftpServer.DialWithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	err = connection.Login(viper.GetString("ftp.user"), viper.GetString("ftp.password"))
	if err != nil {
		return nil, err
	}

	return connection, nil
}
