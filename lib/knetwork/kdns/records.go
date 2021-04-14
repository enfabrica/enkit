package kdns

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/miekg/dns"
	"strings"
)

type RecordControllerErr error

var (
	UnSupportedTypeErr = errors.New("type of record not supported")
)

type RecordOp uint16

var (
	RecordDelete     RecordOp = 1
	RecordWrite      RecordOp = 2
	RecordWriteForce RecordOp = 3
	RecordEdit       RecordOp = 4
	Shutdown         RecordOp = 5
)

// RecordController is a controller that specifically controls a single domain. It handles adding, removing and editing
// dns records in place and should never error out. All methods write errors to the controllers error channel, and operate on
// a fire and forget methodology.
type RecordController struct {
	Log logger.Logger

	aRecords   chan []dns.RR
	aRecordOps chan recordOperation

	txtRecords   chan []dns.RR
	txtRecordOps chan recordOperation
}

type recordOperation struct {
	Type     RecordOp
	Record   dns.RR
	Records  []dns.RR
	Keywords []string
}

// start will spin up the controller and begin handling dns data requests. It is non blocking and is called on NewRecordController
func (rc *RecordController) start() {
	go rc.watchARecords()
	go rc.watchTxtRecords()
}

func handleOperation(src []dns.RR, operation recordOperation) []dns.RR {
	switch operation.Type {
	case RecordWrite:
		return append(src, operation.Record)
	case RecordEdit:
		return append(DeleteIfContains(operation.Keywords, src), operation.Records...)
	case RecordDelete:
		return DeleteIfContains(operation.Keywords, src)
	case RecordWriteForce:
		return operation.Records
	}
	return src
}

func (rc *RecordController) watchARecords() {
	var aRecords []dns.RR
	defer rc.Log.Warnf("exiting a records")
	defer close(rc.aRecords)
	defer close(rc.aRecordOps)
	for {
		select {
		case operation := <-rc.aRecordOps:
			if operation.Type == Shutdown {
				rc.Log.Warnf("shutting down A record server")
				return
			}
			aRecords = handleOperation(aRecords, operation)
		case rc.aRecords <- aRecords:
		}
	}
}

func (rc *RecordController) watchTxtRecords() {
	var txtRecords []dns.RR
	for {
		select {
		case operation := <-rc.txtRecordOps:
			if operation.Type == Shutdown {
				return
			}
			txtRecords = handleOperation(txtRecords, operation)
		case rc.txtRecords <- txtRecords:
		}
	}
}

// FetchRecords will return the []dns.RR of whatever records type is inputted. If the controller does not recognize the
// record type, or if no records of that type exists, it will return an empty list. It takes in a dns.Type.
func (rc *RecordController) FetchRecords(t uint16) []dns.RR {
	switch t {
	case dns.TypeA:
		return <-rc.aRecords
	case dns.TypeTXT:
		return <-rc.txtRecords
	default:
		return []dns.RR{}
	}
}

// AddRecords will arbitrarily add records to the controller if they match the origin of the controller as well as if they
// are of the currently support record type. If any errors occur, it will write to the general error channel
func (rc *RecordController) AddRecords(rr []dns.RR) {
	for _, r := range rr {
		switch r.Header().Rrtype {
		case dns.TypeA:
			rc.aRecordOps <- recordOperation{Type: RecordWrite, Record: r}
		case dns.TypeTXT:
			rc.txtRecordOps <- recordOperation{Type: RecordWrite, Record: r}
		default:
			rc.Log.Errorf("%s", fmt.Errorf("kdns currently does not support record type %s: full record: %v",
				r.Header().String(), r.String()))
		}
	}
}

// SetRecords will hard replace records of type recordType in controller.
// are of the currently support record type. If any errors occur, it will write to the general error channel
func (rc *RecordController) SetRecords(rr []dns.RR) {
	var txtRecords []dns.RR
	var aRecords []dns.RR
	for _, r := range rr {
		switch r.Header().Rrtype {
		case dns.TypeA:
			aRecords = append(aRecords, r)
		case dns.TypeTXT:
			txtRecords = append(txtRecords, r)
		default:
			rc.Log.Errorf("%s", fmt.Errorf("kdns currently does not support record type %s: full record: %v", r.Header().String(), r.String()))
		}
	}
	if len(txtRecords) != 0 {
		rc.txtRecordOps <- recordOperation{Type: RecordWriteForce, Records: txtRecords}
	}
	if len(aRecords) != 0 {
		rc.aRecordOps <- recordOperation{Type: RecordWriteForce, Records: aRecords}
	}
}

// EditRecords will replace records that match the keywords. if the supplied records also match
// are of the currently support record type. If any errors occur, it will write to the general error channel
func (rc *RecordController) EditRecords(rrs []dns.RR, keywords []string) {
	var txtRecords []dns.RR
	var aRecords []dns.RR
	for _, r := range rrs {
		switch r.Header().Rrtype {
		case dns.TypeA:
			aRecords = append(aRecords, r)
		case dns.TypeTXT:
			txtRecords = append(txtRecords, r)
		default:
			rc.Log.Errorf("%s", fmt.Errorf("%w: %v", UnSupportedTypeErr, r.Header().Rrtype))
		}
	}
	if len(aRecords) != 0 {
		rc.aRecordOps <- recordOperation{Type: RecordEdit, Records: aRecords, Keywords: keywords}
	}
	if len(txtRecords) != 0 {
		rc.txtRecordOps <- recordOperation{Type: RecordEdit, Records: txtRecords, Keywords: keywords}
	}
}

// DeleteRecords will delete records that match the keywords provided. Case sensitive/no processing is done
// TODO(adam): support regex?
func (rc *RecordController) DeleteRecords(keywords []string, recordType uint16) {
	switch recordType {
	case dns.TypeA:
		rc.aRecordOps <- recordOperation{Type: RecordDelete, Keywords: keywords}
	case dns.TypeTXT:
		rc.txtRecordOps <- recordOperation{Type: RecordDelete, Keywords: keywords}
	default:
		rc.Log.Errorf("%s", fmt.Errorf("%w: %v", UnSupportedTypeErr, recordType))
	}
}

// NewRecordController create a new controller for a specified origin, or basename;
func NewRecordController(l logger.Logger) *RecordController {
	rc := &RecordController{
		aRecordOps:   make(chan recordOperation),
		aRecords:     make(chan []dns.RR),
		txtRecordOps: make(chan recordOperation),
		txtRecords:   make(chan []dns.RR),
		Log:          l,
	}
	rc.start()
	return rc
}

func DeleteIfContains(keywords []string, src []dns.RR) []dns.RR {
	var toReturn []dns.RR
	for _, r := range src {
		shouldDel := false
		for _, sv := range keywords {
			if strings.Contains(r.String(), sv) {
				shouldDel = true
			}
		}
		if !shouldDel {
			toReturn = append(toReturn, r)
		}
	}
	return toReturn
}

func (rc RecordController) Close() {
	rc.aRecordOps <- recordOperation{Type: Shutdown}
	rc.txtRecordOps <- recordOperation{Type: Shutdown}
}
