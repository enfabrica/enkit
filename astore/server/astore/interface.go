package astore

import (
	"time"

	apb "github.com/enfabrica/enkit/astore/proto"
)

const KindArtifact = "Artifact"

type Artifact struct {
	Uid string
	Sid string
	Tag []string

	MD5  []byte
	Size int64

	Parent  string
	Creator string
	Created time.Time
	Note    string `datastore:",noindex"`
}

func (af *Artifact) ToProto(arch string) *apb.Artifact {
	return &apb.Artifact{
		Uid:          af.Uid,
		Sid:          af.Sid,
		Architecture: arch,
		MD5:          af.MD5,
		Size:         af.Size,
		Tag:          af.Tag,
		Creator:      af.Creator,
		Created:      af.Created.UnixNano(),
		Note:         af.Note,
	}
}

const KindArchitecture = "Arch"

type Architecture struct {
	Parent string

	Created time.Time
	Creator string
}

const KindPathElement = "Pel"

type PathElement struct {
	Parent string

	Created time.Time
	Creator string
}

const KindPublished = "Pub"

type Published struct {
	Parent  string
	Creator string
	Created time.Time

	// Fields from RetrieveRequest.
	Uid          string
	Path         string
	Architecture string

	// When converting this to a query, no tags vs empty tag array have different meanings:
	// The former indicates that the client specified no tags to filter by.
	// The latter indicates that the client is looking for artifacts with no tags assigned.
	//
	// We flatten the struct here, so we use a bool to differentiate between the two cases.
	HasTags bool
	Tag     []string
}

func FromListRequest(req *apb.ListRequest, pub *Published) *Published {
	pub.Uid = req.Uid
	pub.Path = req.Path
	pub.Architecture = req.Architecture
	if req.Tag != nil {
		pub.HasTags = true
		pub.Tag = req.Tag.Tag
	}

	return pub
}

func FromRetrieveRequest(req *apb.RetrieveRequest, pub *Published) *Published {
	pub.Uid = req.Uid
	pub.Path = req.Path
	pub.Architecture = req.Architecture
	if req.Tag != nil {
		pub.HasTags = true
		pub.Tag = req.Tag.Tag
	}

	return pub
}

func (pub *Published) ToRetrieveRequest() *apb.RetrieveRequest {
	req := &apb.RetrieveRequest{}
	req.Uid = pub.Uid
	req.Path = pub.Path
	req.Architecture = pub.Architecture
	if pub.HasTags {
		req.Tag = &apb.TagSet{Tag: pub.Tag}
	}

	return req
}

func (pub *Published) ToListRequest() *apb.ListRequest {
	req := &apb.ListRequest{}
	req.Uid = pub.Uid
	req.Path = pub.Path
	req.Architecture = pub.Architecture
	if pub.HasTags {
		req.Tag = &apb.TagSet{Tag: pub.Tag}
	}

	return req
}
