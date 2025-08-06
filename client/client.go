package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
	"util"
)

// Define configuration
type Config struct {
	Client struct {
		Host          string        `yaml:"host"`
		Port          string        `yaml:"port"`
		ClientId      string        `yaml:"clientId"`
		PingMaxCnt    int           `yaml:"pingMaxCnt"`
		PingInterval  time.Duration `yaml:"pingInterval"`
		ReconWaitTime time.Duration `yaml:"reconWaitTime"`
		BufSize       int           `yaml:"bufSize"`
	} `yaml:"client"`
}

// Configuration
var cfg *Config

func ping(conn *net.TCPConn) {
	flag := false // is first connect
	cnt := 0      // ping failure count
	for {
		if flag {
			_, err := conn.Write([]byte("1:PING;"))
			if err != nil {
				cnt++
				if cnt >= cfg.Client.PingMaxCnt {
					log.Printf("Disconnected from the server: %s", err)
					return
				}
			}
			cnt = 0
		} else {
			_, err := conn.Write([]byte("0:" + cfg.Client.ClientId + ";"))
			if err != nil {
				log.Printf("Registration Error: %s", err)
				return
			}
			flag = true
		}
		time.Sleep(cfg.Client.PingInterval)
	}
}

// messageForward Connect the data tunnel for data forwarding.
func messageForward(info string) {
	dataConn, err := util.CreateTCPConnect(cfg.Client.Host, cfg.Client.Port)
	if err != nil {
		return
	}
	infos := strings.Split(info, ":")
	targetConn, err := util.CreateTCPConnect(infos[1], infos[2])
	if err != nil {
		return
	}
	_, err1 := dataConn.Write([]byte("3:" + infos[0] + ";"))
	if err1 != nil {
		log.Printf("Data tunnel connection Error %s", err1)
		return
	}
	log.Printf("Start transmit: %s:%s", infos[1], infos[2])
	go io.Copy(targetConn, dataConn)
	go io.Copy(dataConn, targetConn)
}

func TCPClient() {

	for {
		conn, err := util.CreateTCPConnect(cfg.Client.Host, cfg.Client.Port)
		if err != nil {
			log.Printf(fmt.Sprintf("Error connecting to server: %s", err))
			time.Sleep(cfg.Client.ReconWaitTime)
			continue
		}
		log.Printf("Sever connecting SUCCESS：%s\n", conn.RemoteAddr().String())

		go ping(conn)

		for {
			data, err1 := util.GetDataFromConnection(cfg.Client.BufSize, conn)
			if err1 != nil {
				log.Printf("Disconnected from the server: %s", err1)
				break
			}
			datas := strings.Split(string(data), ";")
			for _, v := range datas {
				if v != "" && strings.HasPrefix(v, "2:") {
					go messageForward(v[2:])
				}
			}
		}
		time.Sleep(cfg.Client.ReconWaitTime)
	}
}

func main() {

	cfg = util.LoadConfig[Config]("./config/app.yml")

	TCPClient()

}
