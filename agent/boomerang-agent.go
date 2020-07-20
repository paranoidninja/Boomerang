package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

var b_logo = `
  ______________________________________________________________
 |                                                              |
 |   (^^^^^^^^^^^^^^^^^^^^)                                     |
 |  (                      )         ########################## |
 | (    boomerang-server    ) <===== #    boomerang-agent     # |
 |  (                      )         ########################## |
 |   (vvvvvvvvvvvvvvvvvvvv)                                     |
 |______________________________________________________________|
`

var agentListenerAddr = flag.String("r", "", "Remote host to connect to, eg: 192.168.0.201:10443")
var log_file = flag.String("o", "", "output for logging")
var verbosity = flag.Bool("v", false, "stdout logs")

const defaultBufSize = 65536

var connectedSockets = 0
var totalBytesIn = 0
var totalBytesOut = 0

type Tunnel struct {
	agentConn     net.Conn
	forwarderConn net.Conn
	closed        bool
}

func accessRefuse(tun *Tunnel) {
	buf := []byte{0, 0x5b, 0, 0, 0, 0, 0, 0}
	tun.agentConn.Write(buf)
	tun.closed = true
}

func accessGrant(tun *Tunnel) {
	buf := []byte{0, 0x5a, 0, 0, 0, 0, 0, 0}
	tun.agentConn.Write(buf)
}

func serveInternalTunnel(tun *Tunnel) {
	defer tun.forwarderConn.Close()
	buf := make([]byte, defaultBufSize)
	for !tun.closed {
		n, err := tun.forwarderConn.Read(buf)
		log.Printf("[+] %v Bytes Read From Service %v", n, tun.forwarderConn.RemoteAddr())
		if n == 0 {
			tun.closed = true
			return
		}

		if err != nil {
			return
		}

		bytesWritten, err := tun.agentConn.Write(buf[:n])
		if err != nil {
			log.Println(err)
		}
		totalBytesOut += bytesWritten
		log.Printf("[+] %v Bytes Written To Boomerang Server %v", bytesWritten, tun.agentConn.RemoteAddr())
	}
}

func connectInternal(buf []byte, bufSize int, tun *Tunnel) bool {
	end := bytes.IndexByte(buf[8:], byte(0))
	if end < -1 {
		log.Printf("[!] cannot find '\\0', need to read more data")
		return false // need to read more data
	}
	ver := buf[0]
	cmd := buf[1]
	port := int(buf[2])<<8 + int(buf[3])
	ip := net.IPv4(buf[4], buf[5], buf[6], buf[7])
	log.Printf("[*] SOCKS=%v CMD=%v, Service Requested %v:%v\n", ver, cmd, ip, port)

	a := &net.TCPAddr{IP: ip, Port: port}

	forwarderConn, err := net.DialTCP("tcp", nil, a)
	if err != nil {
		log.Println(err)
		accessRefuse(tun)
		return false
	}
	log.Printf("\033[32m[*] %s-<><>-%s [Remote Service]\033[37m", *agentListenerAddr, a)
	tun.forwarderConn = forwarderConn
	if end < bufSize {
		bufferWritten, err := tun.forwarderConn.Write(buf[end:])
		if err != nil {
			log.Println(err)
		}
		log.Printf("[.] %v Bytes Written To Service %v", bufferWritten, tun.forwarderConn.RemoteAddr())
	}
	go serveInternalTunnel(tun)
	return true
}

func getService(agentConn net.Conn, buf []byte, bufSize, bytesRead int) {
	tun := new(Tunnel)
	defer agentConn.Close()
	tun.agentConn = agentConn
	isInit := true
	for !tun.closed {
		if !isInit {
			bytesRead, _ = tun.agentConn.Read(buf[bufSize:])
			totalBytesIn += bytesRead
			log.Printf("[+] %v Bytes Read From Boomerang Server %v", bytesRead, tun.agentConn.RemoteAddr())
		}
		isInit = false
		if bytesRead == 0 {
			break
		}

		if tun.forwarderConn != nil {
			bufferWritten, err := tun.forwarderConn.Write(buf[:bytesRead+bufSize])
			if err != nil {
				log.Println(err)
			}
			log.Printf("[.] %v Bytes Written To Service %v", bufferWritten, tun.forwarderConn.RemoteAddr())
			bufSize = 0
			continue
		}

		if bytesRead+bufSize <= 8 {
			log.Printf("[-] Need more than 8 bytes")
			continue
		}

		if connectInternal(buf, bufSize, tun) {
			bufSize = 0
			accessGrant(tun)
		}
	}
	connectedSockets -= 1
}

func printConnections() {
	fmt.Print("\033[s")
	for {
		// fmt.Printf("\033[2K\r Active Sockets: %d, Bytes In: %d, Bytes Out: %d", connectedSockets, totalBytesIn, totalBytesOut)
		fmt.Printf("\033[u\033[K [+] Active Sockets: %d", connectedSockets)
		fmt.Print("\033[B")
		fmt.Printf("\033[2K\r [+] Bytes In: %d", totalBytesIn)
		fmt.Print("\033[B")
		fmt.Printf("\033[2K\r [+] Bytes Out: %d", totalBytesOut)
		time.Sleep(1 * time.Second)
	}
}

func main() {
	flag.Parse()
	// log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	isInit := true

	if *agentListenerAddr == "" {
		flag.PrintDefaults()
		return
	}

	for {
		agentConn, err := net.Dial("tcp", *agentListenerAddr)
		if err != nil {
			log.Println(err)
		}

		if isInit {
			b_logo = strings.Replace(b_logo, "   boomerang-agent     ", fmt.Sprintf("%-23v", agentConn.LocalAddr()), -1)
			b_logo = strings.Replace(b_logo, "   boomerang-server    ", fmt.Sprintf("%-23s", *agentListenerAddr), -1)
			fmt.Printf("\033[32m\u001b[1m")
			fmt.Println(b_logo)
			fmt.Printf("\033[37m\u001b[0m")

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

		}
		isInit = false

		bufSize := 0
		buf := make([]byte, defaultBufSize)
		log.Printf("\033[32m[+] %v-<><>-%s [Boomerang Server]\033[37m", agentConn.LocalAddr(), *agentListenerAddr)
		connectedSockets += 1
		bytesRead, err := agentConn.Read(buf[bufSize:])
		if err != nil {
			log.Println(err)
		}
		log.Printf("[+] %v Bytes Read From Boomerang Server %v", bytesRead, agentConn.RemoteAddr())
		if bytesRead == 0 {
			continue
		}
		go getService(agentConn, buf, bufSize, bytesRead)
	}
}
