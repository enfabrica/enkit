package license

import (
	astoreServer "github.com/enfabrica/enkit/astore/server/astore"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
	"fmt"
)

type LicenseCounter struct {
	counterMutex sync.Mutex
	clientMutex  sync.Mutex
	clients      map[string]*Client  // client connection metadata
	licenses     map[string]*License // current and total licenses available per vendor
	queue        map[string][]string // queue of jobs waiting to be handled
}

func (c *LicenseCounter) increment(vendor string, num int, hash string) int {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()
	if c.licenses[vendor].Used >= c.licenses[vendor].Total {
        log.Printf("Cannot acquire more licenses than available \n")
        return c.licenses[vendor].Used
	}
	log.Printf("License acquired by %s \n", hash)
	c.licenses[vendor].Used += num
	return c.licenses[vendor].Used
}

func (c *LicenseCounter) decrement(vendor string, num int, hash string) int {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()
	if c.licenses[vendor].Used-num < 0 {
		log.Printf("Cannot release a negative number of licenses \n")
        return c.licenses[vendor].Total
	}
	log.Printf("License released by %s \n", hash)
	c.licenses[vendor].Used -= num
	return c.licenses[vendor].Total
}

func (c *LicenseCounter) peekLicense(vendor string) (int, int, bool) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()
	return c.licenses[vendor].Used, c.licenses[vendor].Total, c.licenses[vendor].Used < c.licenses[vendor].Total
}

func (c *LicenseCounter) enqueue(key string, val *Client) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	c.queue[val.Vendor] = append(c.queue[val.Vendor], key)
	c.clients[key] = val
	log.Printf("Client %s from %s@%s waiting for %d %s %s \n",
		key, val.User, val.IP, val.Quantity, val.Vendor, val.Feature)
	log.Printf("%s job queue: %s \n", val.Vendor, strings.Join(c.queue[val.Vendor], ", "))
}

func (c *LicenseCounter) dequeue(key string, vendor string) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	c.queue[vendor] = c.queue[vendor][1:]
	delete(c.clients, key)
	log.Printf("Client %s removed from the %s queue \n", key, vendor)
}

func (c *LicenseCounter) del(key string, vendor string) {
	index := -1
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
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
		log.Printf("%s is not in queue \n", key)
	}
}

func (c *LicenseCounter) validLicense(vendor string, feature string) bool {
	licenses := map[string]map[string]bool{"xilinx": {"vivado": true},
                                           "cadence": {"xcelium": true}}
	_, ok := licenses[vendor][feature]
	return ok
}

func (c *LicenseCounter) peekQueue(vendor string, hash string) bool {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	return c.queue[vendor][0] == hash
}

type Client struct {
	IP       string
	User     string
	Start    time.Time
	Quantity int32
	Vendor   string
	Feature  string
}

type License struct {
	Total int
	Used  int
}

var totalCadenceLic = 3
var totalXilinxLic = 3
var licenseCounter = LicenseCounter{
	licenses: map[string]*License{"xilinx": &License{Used: 0, Total: totalXilinxLic}, "cadence": &License{Used: 0, Total: totalCadenceLic}},
	clients:  map[string]*Client{},
	queue:    map[string][]string{"xilinx": make([]string, 0), "cadence": make([]string, 0)}}

type Server struct {
	rng *rand.Rand
}

func (s *Server) Polling(stream rpc_license.License_PollingServer) error {
	var hash string
	var vendor string
	var status error
	s.rng = rand.New(rand.NewSource(rand.Int63()))
	for {
		recv, err := stream.Recv()
		// remove client hash from queue when connection is broken
		if err == io.EOF {
			licenseCounter.del(hash, vendor)
			return nil
		}
		if err != nil {
			licenseCounter.del(hash, vendor)
			return err
		}
		if !licenseCounter.validLicense(recv.Vendor, recv.Feature) {
			errMsg := fmt.Sprintf("\"%s %s\" is an unsupported vendor feature combination", recv.Vendor, recv.Feature)
			return grpcStatus.Errorf(grpcCodes.InvalidArgument, errMsg)
		}
		md, ok := metadata.FromIncomingContext(stream.Context())
		if !ok {
			log.Printf("grpc metadata not ok \n")
		}
		ip := strings.Join(md[":authority"], " ")
		username := recv.User
		start := time.Now().UTC()
		vendor = recv.Vendor
		if recv.Hash == "" {
			hash, err = astoreServer.GenerateUid(s.rng)
			if err != nil {
				log.Printf("Failed to generate hash for %s@%s", recv.User, ip)
				return err
			}
			client := Client{IP: ip, User: username, Start: start, Quantity: recv.Quantity, Vendor: vendor, Feature: recv.Feature}
			licenseCounter.enqueue(hash, &client)
		} else {
			hash = recv.Hash
		}
		_, total, available := licenseCounter.peekLicense(vendor)
		if available && licenseCounter.peekQueue(vendor, hash) {
			log.Printf("%s %s licenses in use: %d/%d \n", vendor, recv.Feature,
				licenseCounter.increment(vendor, int(recv.Quantity), hash),
				total)
			licenseCounter.dequeue(hash, vendor)
			err := stream.Send(&rpc_license.PollingResponse{Acquired: true, Hash: hash})
			if err != nil {
                status = err
				log.Printf("Failed to send message back to client %s: %s \n", hash, err)
			}
            break
		} else {
			err := stream.Send(&rpc_license.PollingResponse{Acquired: false, Hash: hash})
			if err != nil {
				log.Printf("Failed to send message back to client %s: %s \n", hash, err)
				licenseCounter.del(hash, vendor)
				return err
			}
		}
	}
	// license acquired
	for {
		_, err := stream.Recv()
		if err == io.EOF || err == nil {
			status = nil
			break
		} else {
			status = err
			break
		}
	}
	log.Printf("Client %s disconnected \n", hash)
	_ = licenseCounter.decrement(vendor, 1, hash)
	return status
}
