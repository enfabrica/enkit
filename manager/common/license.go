package license
import (
	"log"
	"sync"
	"context"
	rpc_license "github.com/enfabrica/enkit/manager/rpc"
)

type LicenseCounter struct {
    mutex sync.Mutex
    counter int
    total int
}

func (c *LicenseCounter) increment(num int) int {
    c.mutex.Lock()
    if c.counter < c.total {
        c.counter += num
    }
    c.mutex.Unlock()
    return c.counter
}

func (c *LicenseCounter) decrement(num int) int {
	c.mutex.Lock()
	if c.counter - num >= 0 {
		c.counter -= num
	}
	c.mutex.Unlock()
	return c.counter
}

var totalLicenses = 3
var licenseCounter = LicenseCounter{counter: 0, total: totalLicenses}
type Server struct {
}

func (s *Server) Acquire(ctx context.Context, request *rpc_license.AcquireRequest) (*rpc_license.AcquireResponse, error) {
	log.Printf("Received request for %d %s feature %s \n", request.Quantity, request.Vendor, request.Feature)
	if licenseCounter.counter < licenseCounter.total {
		log.Printf("Licenses in use: %d/%d \n", licenseCounter.increment(int(request.Quantity)), licenseCounter.total)
		return &rpc_license.AcquireResponse{Available: true, Waiting: false, Missing: false}, nil
	} else {
		return &rpc_license.AcquireResponse{Available: false, Waiting: true, Missing: false}, nil
	}
}

func (s *Server) Release(ctx context.Context, request *rpc_license.ReleaseRequest) (*rpc_license.ReleaseResponse, error) {
	log.Printf("Received request to release %d %s feature %s \n", request.Quantity, request.Vendor, request.Feature)
	if licenseCounter.counter - int(request.Quantity) >= 0 {
		log.Printf("Licenses in use: %d/%d \n", licenseCounter.decrement(int(request.Quantity)), licenseCounter.total)
		return &rpc_license.ReleaseResponse{Success: true}, nil
	} else {
		return &rpc_license.ReleaseResponse{Success: false}, nil
	}
}

func (s *Server) KeepAlive(ctx context.Context, request *rpc_license.KeepAliveRequest) (*rpc_license.KeepAliveResponse, error) {
	if request.Heartbeat {
		log.Printf("Received heartbeat \n")
	}
}
