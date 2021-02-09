package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"
)

var cRed = "\033[31m"
var cReset = "\033[0m"
var cGreen = "\033[32m"
var cYellow = "\033[33m"

var boomerang_version = 0.1

var b_logo = `
  _______________________________________________________            _______________________________________________
 |                                                       ` + cRed + `|uuuuuuuuuu|` + cReset + `                                               |
 |                                                       ` + cRed + `|          |` + cReset + `                                               |
 |                                                       ` + cRed + `|          |` + cYellow + `                                     RDP/VNC` + cReset + `   |
 | ` + cYellow + `  _____________           (^^^^^^^^^^^^^^^^)          ` + cRed + `|          |` + cYellow + `                                     /` + cReset + `         |
 | ` + cYellow + `||             ||        (                  )         ` + cRed + `|          |` + cGreen + `        ####################` + cYellow + `        /` + cReset + `          |
 | ` + cYellow + `|| proxychains || ====> (  boomerang-server  )` + cGreen + ` <===== ` + cRed + `| FIREWALL |` + cGreen + ` <===== # boomerang-agent  # ====> ` + cYellow + `{------SSH` + cReset + `  |
 | ` + cYellow + `||_____________||        (                  )         ` + cRed + `|          |` + cGreen + `        ####################` + cYellow + `        \` + cReset + `          |
 | ` + cYellow + `       / \                (vvvvvvvvvvvvvvvv)          ` + cRed + `|          |` + cYellow + `                                     \` + cReset + `         |
 |                                                       ` + cRed + `|          |` + cYellow + `                                     HTTP/FTP` + cReset + `  |
 |                                                       ` + cRed + `|          |` + cReset + `                                               |
 |_______________________________________________________` + cRed + `|..........|` + cReset + `_______________________________________________|
 
`

const defaultBufSize = 65536

var agentAddr = flag.String("r", "", "local ip:port to listen for agent, eg: 192.168.0.201:10443")
var proxyAddr = flag.String("l", "", "local ip:port to listen for proxy client, eg: 192.168.0.201:9050")
var log_file = flag.String("o", "", "output for logging")
var verbosity = flag.Bool("v", false, "stdout logs")

var connectedClients = 0
var totalBytesIn = 0
var totalBytesOut = 0

func tunnelProxy2Agent(proxyConn, agentConn net.Conn) {
	defer proxyConn.Close()
	defer agentConn.Close()
	buf := make([]byte, defaultBufSize)
	bufSize := 0
	for {
		bytesRead, err := proxyConn.Read(buf[bufSize:])
		if err != nil {
			log.Println(err)
			break
		}
		totalBytesIn += bytesRead
		log.Printf("[+] %v Bytes Read From Proxy Client %v", bytesRead, proxyConn.RemoteAddr())
		if bytesRead == 0 {
			log.Println("ret 1")
			break
		}

		bytesWritten, err := agentConn.Write(buf[:bytesRead+bufSize])
		if err != nil {
			log.Println(err)
			break
		}
		log.Printf("[.] %v Bytes Written To Agent %v", bytesWritten, agentConn.RemoteAddr())
	}
	connectedClients -= 1
}

func tunnelAgent2Proxy(proxyConn, agentConn net.Conn) {
	defer proxyConn.Close()
	defer agentConn.Close()
	buf := make([]byte, defaultBufSize)
	bufSize := 0
	for {
		bytesRead, err := agentConn.Read(buf[bufSize:])
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("[+] %v Bytes Read From Agent %v", bytesRead, agentConn.RemoteAddr())
		if bytesRead == 0 {
			log.Println("ret 2")
			return
		}

		bytesWritten, err := proxyConn.Write(buf[:bytesRead+bufSize])
		if err != nil {
			log.Println(err)
			return
		}

		totalBytesOut += bytesWritten
		log.Printf("[.] %v Bytes Written To Proxy Client %v", bytesWritten, proxyConn.RemoteAddr())
	}
}

func printConnections() {
	fmt.Print("\033[s")
	for {
		// fmt.Printf("\033[2K\r Active Sockets: %d, Bytes In: %d, Bytes Out: %d", connectedSockets, totalBytesIn, totalBytesOut)
		fmt.Printf("\033[u\033[K [+] Active Clients: %d", connectedClients)
		fmt.Print("\033[B")
		fmt.Printf("\033[2K\r [+] Bytes In: %d", totalBytesIn)
		fmt.Print("\033[B")
		fmt.Printf("\033[2K\r [+] Bytes Out: %d", totalBytesOut)
		time.Sleep(1 * time.Second)
	}
}

func main() {

	flag.Parse()
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	fmt.Println(b_logo)
	fmt.Printf(" Boomerang Server v%v\n\n", boomerang_version)

	if !*verbosity {
		log.SetOutput(ioutil.Discard)
		go printConnections()
	}

	if *log_file != "" {
		file, err := os.OpenFile(*log_file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Error creating file: %v", err)
		}
		defer file.Close()
		if *verbosity {
			wrt := io.MultiWriter(os.Stdout, file)
			log.SetOutput(wrt)
		} else {
			wrt := io.MultiWriter(file)
			log.SetOutput(wrt)
		}

	}

	if *agentAddr == "" || *proxyAddr == "" {
		flag.PrintDefaults()
		return
	}

	log.Println("[+] Listening Agent Interface", *agentAddr)
	agentListener, err := net.Listen("tcp", *agentAddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("[+] Listening Proxy Interface", *proxyAddr)
	proxyListener, err := net.Listen("tcp", *proxyAddr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		buf := make([]byte, defaultBufSize)
		agentConn, err := agentListener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("[+] Agent Connected-<><>-%v", agentConn.RemoteAddr().String())
		bytesRead, err := agentConn.Read(buf[0:])
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("[I] Agent %v Initiated Connection With %v Bytes", agentConn.RemoteAddr(), bytesRead)

		proxyConn, err := proxyListener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("[+] Proxy Client Connected-<><>-%v", proxyConn.RemoteAddr().String())
		connectedClients += 1
		go tunnelProxy2Agent(proxyConn, agentConn)
		go tunnelAgent2Proxy(proxyConn, agentConn)
	}
}
