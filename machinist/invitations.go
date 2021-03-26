package machinist

type InvitationToken struct {
	Addresses  []string
	Port       int
	CRT        string
	PrivateKey string
	RootCA     string
}

