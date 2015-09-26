package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"
	"encoding/binary"
	"sort"
	"github.com/crackcomm/go-clitable"
	"math/rand"
	"strings"
)
const receiverPort = 8888
const broadcasterPort = 9100
const spamerPort = 9300
const randomerPort = 9400
const deleteOnMissed = 5
const sendInterval = time.Second *5
const receiveInterval = time.Second * 5
const spamInterval = time.Nanosecond * 10
const randomInterval = time.Millisecond


type Timestamp uint64
type AnnounceData struct {
	addr      net.HardwareAddr
	hostname  string
	timestamp Timestamp
}

var receivedData = make(chan AnnounceData, 1000000)
var endian = binary.BigEndian

func randomer() {

	broadcastConn := openUDPConnOrDie(randomerPort)
	defer broadcastConn.Close();

	addr, err := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + strconv.Itoa(receiverPort))
	if err != nil {
		log.Fatalf("Cannot resolve broadcast receiver address: %s", err)
	}


	ticker := time.NewTicker(spamInterval)
	for _ = range ticker.C {
		intLen := rand.Uint32() % 128
		data := make([]byte, intLen * 4)
		for i := uint32(0); i < intLen; i++ {
			endian.PutUint32(data[i*4:], rand.Uint32())
		}
		_, err = broadcastConn.WriteTo(data, addr)
		if err != nil {
			log.Fatalf("Unable to broadcast RANDOM: %s", err)
		}
	}
}

func spammer() {
	hostname := "SpamSpamSpam      SpamSpamSpamSpamSpamSpamSpamSpamSpamSpamSpamSpam";

	broadcastConn := openUDPConnOrDie(spamerPort)
	defer broadcastConn.Close();

	addr, err := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + strconv.Itoa(receiverPort))
	if err != nil {
		log.Fatalf("Cannot resolve broadcast receiver address: %s", err)
	}

	hardwareAddr := make([]byte, 6)
	broadcastCommonData := append(hardwareAddr, byte(len(hostname)))
	broadcastCommonData = append(broadcastCommonData, ([]byte)(hostname)...)

	timestampBuff := make([]byte, 4) //32!!!!
	ticker := time.NewTicker(spamInterval)
	for _ = range ticker.C {
		unixTimestamp := Timestamp(time.Now().Unix())
		endian.PutUint32(timestampBuff, uint32(unixTimestamp))
		endian.PutUint32(broadcastCommonData, rand.Uint32())
		endian.PutUint32(broadcastCommonData[2:], rand.Uint32())
		copy(broadcastCommonData[7:], []byte(net.HardwareAddr(broadcastCommonData[0:6]).String()))
		_, err = broadcastConn.WriteTo(append(broadcastCommonData, timestampBuff...), addr)
		if err != nil {
			log.Fatalf("Unable to broadcast SPAM: %s", err)
		}
	}
}

func broadcaster() {
	hostname, err := os.Hostname();
	if err != nil {
		log.Fatalln("gethostname error")
	}
	log.Println("Hostname: ", hostname)

	broadcastConn := openUDPConnOrDie(broadcasterPort)
	defer broadcastConn.Close();

	addr, err := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + strconv.Itoa(receiverPort))
	if err != nil {
		log.Fatalf("Cannot resolve broadcast receiver address: %s", err)
	}

	en0Interface, err := net.InterfaceByName("en0")
	if err != nil {
		log.Fatal("Can't get interface")
	}
	hardwareAddr := en0Interface.HardwareAddr
	log.Println("Addr: ", hardwareAddr)
	broadcastCommonData := append(hardwareAddr, byte(len(hostname)))
	broadcastCommonData = append(broadcastCommonData, ([]byte)(hostname)...)

	timestampBuff := make([]byte, 4) //32!!!!
	ticker := time.NewTicker(sendInterval)
	for _ = range ticker.C {
		log.Println("send package")

		unixTimestamp := Timestamp(time.Now().Unix())
		endian.PutUint32(timestampBuff, uint32(unixTimestamp))
		log.Println("timestamp: ", endian.Uint32(timestampBuff))

		_, err = broadcastConn.WriteTo(append(broadcastCommonData, timestampBuff...), addr)
		if err != nil {
			log.Fatalf("Unable to broadcast: %s", err)
		}
	}
}

func receiver() {
	receiverConn := openUDPConnOrDie(receiverPort)
	defer receiverConn.Close()
	buff := make([]byte, 512)
	var spamCount uint64
	for {
		n, _, err := receiverConn.ReadFromUDP(buff)
		if err != nil {
			log.Fatalf("Cannot serve request: %s", err)
		}
		if (n < 10) {
			log.Printf("Illegal format, only %d bytes", n)
			continue
		}
		const hardwareAddrLen = 6

		hardwareAddr := net.HardwareAddr(make([]byte, hardwareAddrLen));
		copy(hardwareAddr, buff[0:hardwareAddrLen]);
		hostnameLen := int(buff[hardwareAddrLen])
		hostnameBegin := hardwareAddrLen + 1
		hostnameEnd := hostnameBegin + hostnameLen
		timeStampLen := n - hostnameEnd
		if timeStampLen != 4 && timeStampLen != 8 {
			log.Printf("Illegal format, message is  %d bytes, where hostnameLen is %d", n, hostnameLen)
			continue
		}
		hostname := string(buff[hostnameBegin:hostnameEnd])

		var timestamp uint64
		timeStampBuff := buff[hostnameEnd:n]
		if timeStampLen == 4 {
			timestamp = uint64(endian.Uint32(timeStampBuff))
		} else {
			timestamp = endian.Uint64(timeStampBuff)
		}
		if strings.Contains(hostname, "SpamSpam") {
			spamCount++
			if (spamCount ^ 0xffff == 0) {
				log.Print("Spam received: ", spamCount)

			}
			continue
		}
//		log.Printf("MAC: %s, host: %s, timestamp: %d", hardwareAddr.String(), hostname, timestamp)
		announce := AnnounceData{hardwareAddr, hostname, Timestamp(timestamp)}
//		log.Printf("MAC: %s, host: %s, timestamp: %d", announce.addr.String(), announce.hostname, announce.timestamp)
		receivedData <- announce
	}
}

func addrToUint(addr net.HardwareAddr) uint64 {
	b := []byte(addr)
	return (uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40)

}
type uint64slice []uint64
func (a uint64slice) Len() int { return len(a) }
func (a uint64slice) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a uint64slice) Less(i, j int) bool { return a[i] < a[j] }

func main() {
	go broadcaster()
	go receiver()
//	go spammer()
//	go randomer()

	ticker := time.NewTicker(receiveInterval)
	var addrDataMap  = make(map[uint64]AnnounceData)
	var addrMissedPacketsMap  = make(map[uint64]int64)
	for _ = range ticker.C {
//		log.Println("output iterate")
		for addrUint, missed := range addrMissedPacketsMap  {
			if missed >= deleteOnMissed {
				delete(addrMissedPacketsMap, addrUint)
				delete(addrDataMap, addrUint)
			} else {
				addrMissedPacketsMap[addrUint] = missed + 1;
			}
		}

		Receive:
		for {
			select {
			case announce := <-receivedData:
				addrUint := addrToUint(announce.addr)
//				log.Printf(
//					"readed from chan. MAC: %s, Key: %d,  host: %s, timestamp: %d",
//					announce.addr,
//					addrUint,
//					announce.hostname,
//					announce.timestamp,
//				)
				//log.Printf("Received from %s, key is: %d", announce.addr, addrUint)
				addrDataMap[addrUint] = announce
				missed, ok := addrMissedPacketsMap[addrUint]
				if ok {
					addrMissedPacketsMap[addrUint] = missed - 1;
				} else {
					addrMissedPacketsMap[addrUint] = 0;
				}
			default:
				break Receive;
			}
		}

		sortedAddrUint := make([]uint64, 0, len(addrDataMap))
		for addrUint, _ := range addrMissedPacketsMap {
			sortedAddrUint = append(sortedAddrUint, addrUint)
		}
		sort.Sort(uint64slice(sortedAddrUint))

		table := clitable.New([]string{"MAC", "Host", "Timestamp", "Missed"})
		for _, addrUint := range sortedAddrUint {
			announce := addrDataMap[addrUint]
			missed, ok := addrMissedPacketsMap[addrUint]
			if !ok {
				log.Fatal("No missed for: ", addrUint)
			}
//			log.Printf(
//				"after sort. MAC: %s, host: %s, timestamp: %d, missed:%d",
//				announce.addr,
//				announce.hostname,
//				announce.timestamp,
//				missed,
//			)
			table.AddRow(map[string]interface{}{
				"MAC" : announce.addr,
				"Host" : announce.hostname,
				"Timestamp" : announce.timestamp,
				"Missed" : missed,
			})
		}
		table.Print()
	}
}

func openUDPConnOrDie(port int) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("udp4", ":" + strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Cannot resolve address: %s", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Fatalf("Cannot listen to address %s: %s", addr, err)
	}
	return conn
}
