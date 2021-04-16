package license

import (
	astoreServer "github.com/enfabrica/enkit/astore/server/astore"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type LicenseCounter struct {
	counterMutex sync.Mutex
	counter      map[string]int // current number of licenses used per vendor
	clientMutex  sync.Mutex
	clients      map[string]*Client  // client connection metadata
	total        map[string]int      // total licenses available per vendor
	queue        map[string][]string // queue of jobs waiting to be handled
}

func (c *LicenseCounter) increment(vendor string, num int, hash string) int {
	c.counterMutex.Lock()
	if c.counter[vendor] < c.total[vendor] {
		log.Printf("License acquired by %s \n", hash)
		c.counter[vendor] += num
	} else {
		log.Fatalf("Cannot acquire more licenses than available \n")
	}
	defer c.counterMutex.Unlock()
	return c.counter[vendor]
}

func (c *LicenseCounter) decrement(vendor string, num int, hash string) int {
	c.counterMutex.Lock()
	if c.counter[vendor]-num >= 0 {
		log.Printf("License released by %s \n", hash)
		c.counter[vendor] -= num
	} else {
		log.Fatalf("Cannot release a negative number of licenses \n")
	}
	defer c.counterMutex.Unlock()
	return c.counter[vendor]
}

func (c *LicenseCounter) enqueue(key string, val *Client) {
	c.clientMutex.Lock()
	c.queue[val.Vendor] = append(c.queue[val.Vendor], key)
	c.clients[key] = val
	log.Printf("Client %s from %s@%s waiting for %d %s %s \n",
		key, val.User, val.IP, val.Quantity, val.Vendor, val.Feature)
	log.Printf("%s job queue: %s \n", val.Vendor, strings.Join(c.queue[val.Vendor], ", "))
	defer c.clientMutex.Unlock()
}

func (c *LicenseCounter) dequeue(key string, vendor string) {
	c.clientMutex.Lock()
	c.queue[vendor] = c.queue[vendor][1:]
	delete(c.clients, key)
	log.Printf("Client %s removed from the %s queue \n", key, vendor)
	defer c.clientMutex.Unlock()
}

func (c *LicenseCounter) del(key string, vendor string) int {
	index := -1
	c.clientMutex.Lock()
	for i := 0; i < len(c.queue[vendor]); i++ {
		if c.queue[vendor][i] == key {
			c.queue[vendor] = append(c.queue[vendor][:i], c.queue[vendor][i+1:]...)
			delete(c.clients, key)
			log.Printf("Job cancelled by client %s - removed from queue \n", key)
			index = i
			break
		}
	}
	if index == -1 {
		log.Fatalf("%s is not in queue \n", key)
	}
	defer c.clientMutex.Unlock()
	return index
}

type Client struct {
	IP       string
	User     string
	Start    time.Time
	Quantity int32
	Vendor   string
	Feature  string
}

var clients = make(map[string]*Client)
var totalCadenceLic = 3
var totalXilinxLic = 3
var licenseCounter = LicenseCounter{
	counter: map[string]int{"xilinx": 0, "cadence": 0},
	total:   map[string]int{"xilinx": totalXilinxLic, "cadence": totalCadenceLic},
	clients: make(map[string]*Client),
	queue:   map[string][]string{"xilinx": make([]string, 0), "cadence": make([]string, 0)}}

type Server struct {
}

func (s *Server) KeepAlive(stream rpc_license.License_KeepAliveServer) error {
	var status error
	var hash string
	var vendor string
	var feature string
	for {
		recv, err := stream.Recv()
		if err == io.EOF {
			status = nil
			break
		}
		if err != nil {
			status = err
			break
		}
		hash, vendor, feature = recv.Hash, recv.Vendor, recv.Feature
		err = stream.Send(&rpc_license.KeepAliveMessage{Hash: hash, Vendor: vendor, Feature: feature})
		if err != nil {
			status = err
			break
		}
	}
	inUse := licenseCounter.decrement(vendor, 1, hash)
	log.Printf("Client %s disconnected \n", hash)
	log.Printf("%s %s licenses in use: %d/%d \n", vendor, feature, inUse, licenseCounter.total[vendor])
	return status
}

func (s *Server) Polling(stream rpc_license.License_PollingServer) error {
	var status error
	var hash string
	var vendor string
	for {
		recv, err := stream.Recv()
		if err == io.EOF {
			status = nil
			break
		}
		if err != nil {
			status = err
			break
		}
		md, ok := metadata.FromIncomingContext(stream.Context())
		if !ok {
			log.Fatalf("grpc metadata not ok \n")
		}
		ip := strings.Join(md[":authority"], " ")
		username := recv.User
		start := time.Now().UTC()
		vendor = recv.Vendor
		if recv.Hash == "" {
			rng := rand.New(rand.NewSource(rand.Int63()))
			hash, err = astoreServer.GenerateUid(rng)
			if err != nil {
				log.Fatalf("Failed to generate hash for %s@%s", recv.User, ip)
			}
			client := Client{IP: ip, User: username, Start: start, Quantity: recv.Quantity, Vendor: vendor, Feature: recv.Feature}
			licenseCounter.enqueue(hash, &client)
		} else {
			hash = recv.Hash
		}
		if licenseCounter.counter[vendor] < licenseCounter.total[vendor] && licenseCounter.queue[vendor][0] == hash {
			log.Printf("%s %s licenses in use: %d/%d \n", vendor, recv.Feature,
				licenseCounter.increment(vendor, int(recv.Quantity), hash),
				licenseCounter.total[vendor])
			licenseCounter.dequeue(hash, vendor)
			return stream.Send(&rpc_license.PollingResponse{Acquired: true, Hash: hash})
		} else {
			err := stream.Send(&rpc_license.PollingResponse{Acquired: false, Hash: hash})
			if err != nil {
				log.Fatalf("Failed to send message back to client: %s \n", err)
			}
		}

	}
	// remove client hash from queue when connection is broken
	licenseCounter.del(hash, vendor)
	return status
}
