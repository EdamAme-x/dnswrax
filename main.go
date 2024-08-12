package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	targetDNS = "8.8.8.8:53" // DNS Server
	listenAddr = "127.0.0.1:53"
)

func main() {
	ln, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		log.Fatal("failed to create UDP listener:", err)
	}
	defer ln.Close()

	log.Printf("DNS Proxy listening on %s", listenAddr)

	for {
		buffer := make([]byte, 512)

		size, addr, err := ln.ReadFrom(buffer)
		if err != nil {
			log.Println("failed to read from client:", err)
			continue
		}

		go func(data []byte, clientAddr net.Addr) {
			lastIndex := 0

			for i := 12;i<512; i++ {
				if data[i] == 0 {
					lastIndex = i
					break
				}
			}

			originalDomain := make([]byte, lastIndex - 12)
			copy(originalDomain, data[12:lastIndex])
			targetDomain, err := decodeDomain(originalDomain)

			if err != nil {
				log.Println("failed to decode domain:", err)
				return
			}

			targetDomain = strings.TrimSuffix(targetDomain, ".proxy")

			head := data[:12]
			tail := data[lastIndex+1:]

			encodedDomain := encodeDomain(targetDomain)

			data = append(head, append(encodedDomain, tail...)...)

			conn, err := net.Dial("udp", targetDNS)
			if err != nil {
				log.Println("failed to connect to DNS server:", err)
				return
			}
			defer conn.Close()

			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Println("failed to set deadline:", err)
				return
			}

			_, err = conn.Write(data)
			if err != nil {
				log.Println("failed to send request to DNS server:", err)
				return
			}

			response := make([]byte, 512)
			_, err = conn.Read(response)

			// response = replaceBytes(response, encodedDomain, originalDomain)

			if err != nil {
				log.Println("failed to read response from DNS server:", err)
				return
			}

			_, err = ln.WriteTo(response, clientAddr)
			if err != nil {
				log.Println("failed to send response to client:", err)
				return
			}
		}(buffer[:size], addr)
	}
}

func encodeDomain(domain string) []byte {
	splittedDomain := strings.Split(domain, ".")
	encodedDomain := []byte{}

	for _, part := range splittedDomain {
		encodedDomain = append(encodedDomain, byte(len(part)))
		encodedDomain = append(encodedDomain, []byte(part)...)
	}

	encodedDomain = append(encodedDomain, 0x00)

	return encodedDomain
}

func decodeDomain(domain []byte) (string, error) {
	domains := []string{}
	for i := 0; i < len(domain); i++ {
		if domain[i] == 0x00 {
			break
		}

		weight := int(domain[i])

		if i+1+weight > len(domain) {
			return "", fmt.Errorf("invalid domain length")
		}

		segment := string(domain[i+1 : i+1+weight])

		domains = append(domains, segment)
		i += weight
	}

	return strings.Join(domains, "."), nil
}
