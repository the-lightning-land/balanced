package bdb

type Invoice struct {
	Destination     string
	PaymentHash     string
	NumSatoshis     int64
	Timestamp       int64
	Expiry          int64
	Description     string
	DescriptionHash string
	FallbackAddr    string
	CltvExpiry      int64
}
