package license
import (
	"io"
	"log"
	"sync"
	"time"
	"strings"
	"math/rand"
	"google.golang.org/grpc/metadata"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
	astoreServer "github.com/enfabrica/enkit/astore/server/astore"
)

type LicenseCounter struct {
    counterMutex sync.Mutex
    counter map[string]int // current number of licenses used per vendor
	clientMutex sync.Mutex
	clients map[string]*Client // client connection metadata
    total map[string]int // total licenses available per vendor
	queue []string
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
	if c.counter[vendor] - num >= 0 {
		log.Printf("License released by %s \n", hash)
		c.counter[vendor] -= num
	} else {
		log.Fatalf("Cannot release a negative number of licenses \n")
	}
	defer c.counterMutex.Unlock()
	return c.counter[vendor]
}

func (c *LicenseCounter) enqueue(key string, val *Client) int {
	c.clientMutex.Lock()
	c.queue = append(c.queue, key)
	c.clients[key] = val
	defer c.clientMutex.Unlock()
	return len(c.queue)
}

func (c *LicenseCounter) dequeue(key string) {
	c.clientMutex.Lock()
	c.queue = c.queue[1:]
	delete(c.clients, key)
	defer c.clientMutex.Unlock()
}

func (c *LicenseCounter) del(key string) int {
	index := -1
	c.clientMutex.Lock()
	for i := 0; i < len(c.queue); i++ {
		if c.queue[i] == key {
			c.queue = append(c.queue[:i], c.queue[i+1:]...)
			delete(c.clients, key)
			log.Printf("Client %s removed from queue \n", key)
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
	IP string
	User string
	Start time.Time
}

var clients = make(map[string]*Client)
var totalCadenceLic = 3
var totalXilinxLic = 3
var licenseCounter = LicenseCounter{counter: map[string]int{"xilinx": 0, "cadence": 0},
									total: map[string]int{"xilinx": totalXilinxLic, "cadence": totalCadenceLic},
									clients: make(map[string]*Client),
									queue: make([]string, 0)}
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
	log.Printf("%s %s licenses in use: %d/%d \n", vendor, feature, inUse, licenseCounter.total[vendor])
	return status
}

func (s *Server) Polling(stream rpc_license.License_PollingServer) error {
	var status error
	var hash string
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
		if recv.Hash == "" {
			rng := rand.New(rand.NewSource(rand.Int63()))
			hash, err = astoreServer.GenerateUid(rng)
			if err != nil {
				log.Fatalf("Failed to generate hash for %s@%s", recv.User, ip)
			}
			length := licenseCounter.enqueue(hash, &Client{IP: ip, User: username, Start: start})
			log.Printf("Client %s from %s@%s received at %s waiting at position %d \n", hash, username, ip, start, length)
		} else {
			hash = recv.Hash
		}
		log.Printf("Received request for %d %s feature %s from client %s \n", recv.Quantity, recv.Vendor, recv.Feature, hash)
		if licenseCounter.counter[recv.Vendor] < licenseCounter.total[recv.Vendor] && licenseCounter.queue[0] == hash {
			log.Printf("%s %s licenses in use: %d/%d \n", recv.Vendor, recv.Feature,
													      licenseCounter.increment(recv.Vendor, int(recv.Quantity), hash),
													      licenseCounter.total[recv.Vendor])
			licenseCounter.dequeue(hash)
			log.Printf("Client %s removed from the queue \n", hash)
			return stream.Send(&rpc_license.PollingResponse{Acquired: true, Hash: hash})
		} else {
			log.Printf("Client %s waiting for %d %s feature %s to be available \n", hash, recv.Quantity, recv.Vendor, recv.Feature)
			err := stream.Send(&rpc_license.PollingResponse{Acquired: false, Hash: hash})
			if err != nil {
				log.Fatalf("Failed to send message back to client: %s \n", err)
			}
		}

	}
	// remove client hash from queue when connection is broken
	licenseCounter.del(hash)
	return status
}
