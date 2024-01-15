package main

import (
	"bytes"
	"os"

	// "go/types"
	// "bytes"
	"context"
	// "encoding/json"
	"encoding/gob"
	"fmt"
	"net"
	"time"

	"github.com/joho/godotenv"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

const googleDNSAddr = "8.8.8.8:53"
const cacheTTL = 60 * time.Second

type CachedDNSResponse struct {
	Header   *dns.MsgHdr
	Answer   []string
	Question string
}

var ctx = context.Background()

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		return
	}
	var REDIS_URI = os.Getenv("REDIS_URI")
	opt, err := redis.ParseURL(REDIS_URI)
	if err != nil {
		panic(err)
	}

	rclient := redis.NewClient(opt)
	// fmt.Println("this is clietnt", rclient)

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1024")
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		return
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		return
	}
	defer conn.Close()

	fmt.Println("UDP DNS server listening on", udpAddr.String())

	buffer := make([]byte, 1024)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}

		fmt.Printf("Received DNS query from %s\n", addr.String())

		handleDNSQuery(conn, rclient, buffer[:n], addr)
		// fmt.Println("this is responseData", buffer[:n])
		// _, err = conn.WriteToUDP(responseData, addr)
		if err != nil {
			fmt.Println("Error writing to UDP:", err)
		}

		fmt.Printf("Sent DNS response to %s\n", addr.String())
	}
}

func handleDNSQuery(conn *net.UDPConn, rclient *redis.Client, data []byte, udpAddr *net.UDPAddr) {
	// Parse the DNS query using the dns library
	msg := new(dns.Msg)
	err := msg.Unpack(data)
	if err != nil {
		fmt.Println("Error unpacking DNS query:", err)
		return
	}
	// fmt.Printf("this is message id %T\n\n\n\n", msg.Id)
	cachedResponse, err := getFromCache(rclient, msg.Question[0].Name, msg.Id)
	// cachedResponse.Id = msg.Id
	if err == nil {
		fmt.Printf("Cache hit for %s\n", msg.Question[0].Name)
		fmt.Println("\n\nCACHED RESPONSE\n\n", cachedResponse)
		sendResponse(conn, udpAddr, cachedResponse)
		return
	}

	response, err := resolveWithGoogleDNS(msg)
	if err != nil {
		fmt.Println("Error resolving query with Google DNS:", err)
		return
	}

	err = storeInCache(rclient, msg.Question[0].Name, response)
	if err != nil {
		fmt.Printf("Error storing in cache: %v\n", err)
	}
	fmt.Println("\n\nDATA STORED IN REDIS CACHE \n\n")
	// Send the DNS response to the client
	sendResponse(conn, udpAddr, response)
}

func resolveWithGoogleDNS(query *dns.Msg) (*dns.Msg, error) {
	client := dns.Client{Net: "udp", Timeout: time.Second}

	// Query Google DNS with the original question
	response, _, err := client.Exchange(query, googleDNSAddr)
	if err != nil {
		return nil, fmt.Errorf("error resolving with Google DNS: %w", err)
	}

	return response, nil
}

func getFromCache(rclient *redis.Client, key string, msgId uint16) (*dns.Msg, error) {
	data, err := rclient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, fmt.Errorf("error retrieving from cache: %w", err)
	}

	// Deserialize the response
	var cachedResponse CachedDNSResponse
	err = unmarshalBinary(data, &cachedResponse)
	if err != nil {
		return nil, fmt.Errorf("error retrieving from cache: %w", err)
	}
	msg := new(dns.Msg)

	// Set the ID to a value of your choice
	msg.Id = msgId

	// Create a question for the dns.Msg
	question := &dns.Question{
		Name:   cachedResponse.Question,
		Qtype:  dns.TypeA,     // Assuming A record type, modify accordingly
		Qclass: dns.ClassINET, // Assuming IN class, modify accordingly
	}
	msg.Question = []dns.Question{*question}

	// Add Answer RRs to the Answer section
	for _, ipStr := range cachedResponse.Answer {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address in cached response: %s", ipStr)
		}

		// Create an A record with the parsed IP
		rr := &dns.A{
			Hdr: dns.RR_Header{Name: cachedResponse.Question, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   ip,
		}

		// Add the A record to the Answer section
		msg.Answer = append(msg.Answer, rr)
	}

	return msg, nil

}

func storeInCache(rclient *redis.Client, key string, response *dns.Msg) error {
	// fmt.Println("this is response", response)
	cachedResponse := CachedDNSResponse{
		Header: &dns.MsgHdr{

			Response:         response.Response, // Set the Response bit to indicate that it's a response
			Opcode:           dns.OpcodeQuery,
			Rcode:            dns.RcodeSuccess,
			RecursionDesired: response.RecursionDesired,
		},
		Answer:   getAnswer(response),
		Question: response.Question[0].Name,
	}

	// Serialize the simplified response to a byte slice
	data, err := marshalBinary(cachedResponse)
	if err != nil {
		return fmt.Errorf("error encoding DNS response: %w", err)
	}

	return rclient.SetEx(ctx, key, data, cacheTTL).Err()
}

func getAnswer(response *dns.Msg) []string {
	var answer []string
	for _, rr := range response.Answer {
		if a, ok := rr.(*dns.A); ok {
			answer = append(answer, a.A.String())
		}
		// Add more checks for other record types if needed
	}
	return answer
}

func marshalBinary(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("error encoding data: %w", err)
	}
	return buf.Bytes(), nil
}

func unmarshalBinary(data []byte, v interface{}) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(v)
	if err != nil {
		return fmt.Errorf("error decoding data: %w", err)
	}
	return nil
}

func sendResponse(conn *net.UDPConn, addr *net.UDPAddr, response *dns.Msg) {
	responseData, err := response.Pack()
	if err != nil {
		fmt.Println("Error packing DNS response:", err)
		return
	}

	_, err = conn.WriteToUDP(responseData, addr)
	if err != nil {
		fmt.Println("Error writing to UDP:", err)
		return
	}

	fmt.Printf("Sent DNS response to %s\n", addr.String())
}
