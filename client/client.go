package main

import (
	"fmt"
	"io"
	"log"
	"net"
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
			_, err := conn.Write([]byte{util.HEARTBEAT})
			if err != nil {
				cnt++
				if cnt >= cfg.Client.PingMaxCnt {
					log.Printf("Disconnected from the server: %s", err)
					return
				}
			}
			cnt = 0
		} else {
			data := make([]byte, len(cfg.Client.ClientId)+1)
			data[0] = util.CONNECT
			copy(data[1:], cfg.Client.ClientId)
			_, err := conn.Write(data)
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
func messageForward(sessionId string, tHost string, tPort string) {
	dataConn, err := util.CreateTCPConnect(cfg.Client.Host, cfg.Client.Port)
	if err != nil {
		return
	}
	targetConn, err := util.CreateTCPConnect(tHost, tPort)
	if err != nil {
		return
	}
	data := make([]byte, 37)
	data[0] = util.C_TO_S
	copy(data[1:], sessionId)
	_, err1 := dataConn.Write(data)
	if err1 != nil {
		log.Printf("Data tunnel connection Error %s", err1)
		return
	}
	go func() {
		n, _ := io.Copy(targetConn, dataConn)
		log.Printf("[%s] S -> T len= %d B", sessionId, n)
	}()
	go func() {
		n, _ := io.Copy(dataConn, targetConn)
		log.Printf("[%s] T -> S len= %d B", sessionId, n)
	}()
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

			n := len(data)
			for i := 0; i < n; {
				flag := data[i]
				i++
				if flag == util.S_TO_C {
					sessionId := string(data[i : i+36])
					tHostLen := int(data[i+36])
					tHost := string(data[i+37 : i+37+tHostLen])
					tPortLen := int(data[i+37+tHostLen])
					tPort := string(data[i+38+tHostLen : i+38+tHostLen+tPortLen])
					i = i + 38 + tHostLen + tPortLen
					go messageForward(sessionId, tHost, tPort)
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
