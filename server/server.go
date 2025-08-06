package main

import (
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"util"
)

// Define configuration
type Config struct {
	Server struct {
		Port    string `yaml:"port"`
		Clients []struct {
			ClientID string `yaml:"clientId"`
			Port     string `yaml:"port"`
			THost    string `yaml:"tHost"`
			TPort    string `yaml:"tPort"`
		} `yaml:"clients"`
		BufSize int `yaml:"bufSize"`
	} `yaml:"server"`
}

// wg Used to wait for all coroutines to finish.
var wg sync.WaitGroup

// Used to store the mapping of ClientID and the corresponding signaling channel for that ClientID.
var clientConnMap map[string]*net.TCPConn

// Cache each session of the user
// Each time the user accesses the server, a SessionID is assigned and a TCP connection is established.
// This mapping stores the relationship between the SessionID and the TCP connection.
var sessionConnMap map[string]*net.TCPConn

// Configuration
var cfg *Config

// initialize
func initialize() {
	clientConnMap = make(map[string]*net.TCPConn)
	sessionConnMap = make(map[string]*net.TCPConn)
}

// clientListen Listen to client registration
func clientListen() {
	tcpListener, err := util.CreateTCPListen("", cfg.Server.Port)
	if err != nil {
		log.Printf("Client listen Error: %s\n", err.Error())
		panic(err)
		return
	}
	log.Printf("Client Listen SUCCESS: %s\n", tcpListener.Addr().String())
	log.Printf("Waiting for client connection...\n")
	for {
		serverConn, err1 := tcpListener.AcceptTCP()
		if err1 != nil {
			log.Printf("Client connection Error：%s\n", err1.Error())
			continue
		}
		go pong(serverConn)
	}
}

func pong(conn *net.TCPConn) {
	for {
		data, err := util.GetDataFromConnection(cfg.Server.BufSize, conn)
		if err != nil {
			log.Printf("Client data read Error：%s\n", conn.RemoteAddr().String())
			return
		}
		msg := string(data)
		if strings.HasPrefix(msg, "0:") {
			f1 := false
			for _, client := range cfg.Server.Clients {
				if client.ClientID == msg[2:len(msg)-1] {
					clientConnMap[msg[2:len(msg)-1]] = conn
					f1 = true
				}
			}
			if !f1 {
				_, _ = conn.Write([]byte("ClientID [" + msg[2:len(msg)-1] + "] not registered"))
				_ = conn.Close()
				return
			}
			log.Printf("Client connection SUCCESS：%s\n", conn.RemoteAddr().String())
		} else if msg == "1:PING;" {
			_, err1 := conn.Write([]byte("1:PONG;"))
			if err1 != nil {
				return
			}
		} else if strings.HasPrefix(msg, "3:") {
			uconn := sessionConnMap[msg[2:len(msg)-1]]
			go io.Copy(conn, uconn)
			go io.Copy(uconn, conn)
			return
		}
	}
}

// portListen Listen to user connections from the ports specified in the configuration file.
func portListen(port string, clientID string, thost string, tport string) {
	tcpListener, err := util.CreateTCPListen("", port)
	if err != nil {
		log.Printf("User listen Error:%s\n", err)
		panic(err)
		return
	}
	log.Printf("User listen SUCCESS：%s\n", tcpListener.Addr().String())
	for {
		userConn, err1 := tcpListener.AcceptTCP()
		if err1 != nil {
			log.Printf("User connect Error：%s\n", err1.Error())
			return
		}
		log.Printf("User connect SUCCESS：%s\n", userConn.RemoteAddr().String())
		sessionID := util.GenerateUUID()
		sessionConnMap[sessionID] = userConn

		// Notify the client to establish a data channel.
		// sessionID:IP:Port
		msg := "2:" + sessionID + ":" + thost + ":" + tport + ";"
		_, err2 := clientConnMap[clientID].Write([]byte(msg))
		if err2 != nil {
			log.Printf("Send msg Error：%s\n", err2.Error())
		}
	}
}

func main() {

	initialize()

	cfg = util.LoadConfig[Config]("./config/app.yml")

	go clientListen()

	for _, client := range cfg.Server.Clients {
		go portListen(client.Port, client.ClientID, client.THost, client.TPort)
	}

	wg.Add(1)
	wg.Wait()
}
