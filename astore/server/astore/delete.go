package astore

import (
	"cloud.google.com/go/datastore"
	"context"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"strconv"
	"time"
)

const (
	FilterForSID     = "Sid = "
	FilterForUID     = "Uid = "
)

type Identifier string

var (
	SIDIdentifier Identifier = "sid"
	UIDIdentifier Identifier = "uid"
)

func (i Identifier) SID() bool {
	return i == SIDIdentifier
}

func (i Identifier) UID() bool {
	return i == UIDIdentifier
}

func (s *Server) Delete(ctx context.Context, deleteRequest *astore.DeleteRequest) (*astore.DeleteResponse, error) {
	//TODO s.List then s.Tag all those that are listed
	identifier, err := detectSIDOrUID([]byte(deleteRequest.Id))
	if err != nil {
		return nil, err
	}
	var listRequest *astore.ListRequest
	var listResponse *astore.ListResponse
	if identifier.UID() {
		listRequest = &astore.ListRequest{Path: objectPath(deleteRequest.Id)}
	} else if identifier.SID() {
		listRequest = &astore.ListRequest{Uid: deleteRequest.Id}
	} else {
		return nil, errors.New("object is not a sid or uid, invalid")
	}
	listResponse, err = s.List(ctx, listRequest)
	if err != nil {
		return nil, err
	}
	for _, artifact := range listResponse.Artifact {
		_, err = s.Note(ctx, &astore.NoteRequest{
			Uid:  artifact.Uid,
			Note: getDeleteString(),
		})
		if err != nil {
			return nil, err
		}
	}
	//dummy
	return &astore.DeleteResponse{

	}, nil
}

//
func detectSIDOrUID(input []byte) (Identifier, error) {
	if len(input) == 34 {
		return SIDIdentifier, nil
	} else if len(input) == 32 {
		return UIDIdentifier, nil
	} else {
		return "", errors.New("this is not a valid uid or sid")
	}
}

//DeleteID will remove from the datastore if a uid is passed in, or if an sid is passed in delete the original from the bucket
// as well as all uid reference in the datastore
func DeleteID(client *datastore.Client, id string) ([]string, error) {
	//TODO support lists of ids to delete?
	identifier, err := detectSIDOrUID([]byte(id))
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	tx, err := client.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	query := datastore.NewQuery(KindArtifact).
		Filter("deleteAt < ", time.Now().Unix()).
		Transaction(tx)

	if identifier.SID() {
		query.Filter(FilterForSID, id)
	} else if identifier.UID() {
		query.Filter(FilterForUID, id)
	}
	var arts []Artifact
	keys, err := client.GetAll(ctx, query, &arts)
	if err != nil {
		return nil, err
	}
	var deletedIDs []string
	for _, k := range keys {
		err = tx.Delete(k)
		if err != nil {
			//add to wrapperand rollback

		}
		deletedIDs = append(deletedIDs, strconv.Itoa(int(k.ID)))
	}
	//todo delete sid from gcs, need to get hands on actual sid an
	if identifier.SID() {
		objectPath(id)
	}
	err = Commit(&tx)
	return deletedIDs, err
}

//todo make time to deletion configurable
func getDeleteString() string {
	return fmt.Sprintf("deleteAt:%d", time.Now().Add(time.Hour*48).Unix())
}
