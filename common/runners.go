package common

import (
	"time"
)

// Layer1Runner implements physical layer tests
type Layer1Runner struct {
	AttemptCount int
}

// Layer2Runner implements data link layer tests
type Layer2Runner struct {
	Targets  []string
	CheckMAC bool
	CheckMTU bool
}

// Layer3Runner implements network layer tests
type Layer3Runner struct {
	Hostname  string
	PingAddr  string
	PingCount int
}

// Layer4Runner implements transport layer tests
type Layer4Runner struct {
	TCPAddresses []string
	UDPAddress   string
	Timeout      time.Duration
}

// Layer5Runner implements session layer tests
type Layer5Runner struct {
	Targets []string
	Timeout time.Duration
}

// Layer6Runner implements presentation layer tests
type Layer6Runner struct {
	DataSets []map[string]string
}

// Layer7Runner implements application layer tests
type Layer7Runner struct {
	Endpoints []string
	Timeout   time.Duration
}
