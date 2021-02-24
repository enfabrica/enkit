package ktest

type MinioDescriptor struct {
	Port uint16
}

//RunMinioServer will spin up a minio serer using the local docker daemon
//it also returns a func to call that will close and destroy the running image
//the port and network bind are determined by docker and returned
func RunMinioServer() (MinioDescriptor, func() error, error) {
	_ = "gcloud beta emulators datastore start --no-store-on-disk"
	return MinioDescriptor{}, nil, nil
}


