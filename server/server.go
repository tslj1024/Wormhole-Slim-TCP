package main

import (
	"io"
	"log"
	"net"
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

var rwLock sync.RWMutex

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
		go handleClientTCPConn(serverConn)
	}
}

func handleClientTCPConn(conn *net.TCPConn) {
	for {
		data, err := util.GetDataFromConnection(cfg.Server.BufSize, conn)
		if err != nil {
			log.Printf("Client data read Error：%s\n", conn.RemoteAddr().String())
			return
		}
		flag := data[0]
		if flag == util.CONNECT {
			f1 := false
			for _, client := range cfg.Server.Clients {
				if client.ClientID == string(data[1:]) {
					rwLock.Lock()
					clientConnMap[string(data[1:])] = conn
					rwLock.Unlock()
					f1 = true
				}
			}
			if !f1 {
				// ClientID is not registered or in valid
				_ = conn.Close()
				return
			}
			log.Printf("Client connection SUCCESS：%s\n", conn.RemoteAddr().String())
		} else if flag == util.HEARTBEAT {
			// PONG
			_, err1 := conn.Write([]byte{util.HEARTBEAT})
			if err1 != nil {
				return
			}
		} else if flag == util.C_TO_S {
			rwLock.RLock()
			uconn := sessionConnMap[string(data[1:])]
			delete(sessionConnMap, string(data[1:]))
			rwLock.RUnlock()
			go func() {
				n, _ := io.Copy(conn, uconn)
				log.Printf("[%s] U -> C len= %d B", string(data[1:]), n)
			}()
			go func() {
				n, _ := io.Copy(uconn, conn)
				log.Printf("[%s] C -> U len= %d B", string(data[1:]), n)
			}()
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
		rwLock.Lock()
		sessionConnMap[sessionID] = userConn
		rwLock.Unlock()

		// Notify the client to establish a data channel.
		// sessionID:IP:Port
		data := make([]byte, cfg.Server.BufSize)
		data[0] = util.S_TO_C
		thostLen := byte(len(thost))
		tportLen := byte(len(tport))
		copy(data[1:], sessionID)
		data[37] = thostLen
		copy(data[38:], thost)
		data[38+thostLen] = tportLen
		copy(data[39+thostLen:], tport)
		rwLock.RLock()
		_, err2 := clientConnMap[clientID].Write(data[:39+thostLen+tportLen])
		rwLock.RUnlock()
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
