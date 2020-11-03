package enslaver

import (
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
)

type Enslaver struct {
}

func (en *Enslaver) Download(*machinist.DownloadRequest, machinist.Enslaver_DownloadServer) error {
	return nil
}

func (en *Enslaver) Upload(machinist.Enslaver_UploadServer) error {
	return nil
}

func (en *Enslaver) Poll(machinist.Enslaver_PollServer) error {
	return nil
}
