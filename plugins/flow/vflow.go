package flow

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"sync"

	"time"

	"observer/store"

	"github.com/EdgeCast/vflow/ipfix"
	netflow9 "github.com/EdgeCast/vflow/netflow/v9"
	"github.com/EdgeCast/vflow/sflow"
)

// IPFlowCollector handles high-velocity UDP ingestion for NetFlow, sFlow, and IPFIX
// It embeds the blazing-fast decoders from EdgeCast/vflow.
type IPFlowCollector struct {
	IPFIXPort   int
	NetFlowPort int
	SFlowPort   int

	// vflow requires memory caches to store the NetFlow/IPFIX Template definitions
	// sent by routers before the actual data payloads arrive.
	ipfixCache   ipfix.MemCache
	netflowCache netflow9.MemCache

	db store.Store
}

// NewCollector creates a new flow listener configuration
func NewCollector(st store.Store) *IPFlowCollector {
	return &IPFlowCollector{
		IPFIXPort:   4739, // Default IPFIX
		NetFlowPort: 2055, // Default NetFlow v9 / v5
		SFlowPort:   6343, // Default sFlow

		// In-memory template caches
		ipfixCache:   ipfix.GetCache("ipfix_templates.cache"),
		netflowCache: netflow9.GetCache("netflow_templates.cache"),

		db: st,
	}
}

// Start launches the UDP listeners simultaneously
func (c *IPFlowCollector) Start() {
	var wg sync.WaitGroup

	wg.Add(3)
	go c.listenIPFIX(&wg)
	go c.listenNetFlow(&wg)
	go c.listenSFlow(&wg)

	log.Println("Nord IPFlow Collector running. Waiting for telemetry...")
	wg.Wait()
}

func (c *IPFlowCollector) listenIPFIX(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: c.IPFIXPort})
	if err != nil {
		log.Fatalf("IPFIX Listen Error: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for IPFIX on UDP :%d", c.IPFIXPort)
	buf := make([]byte, 65535)
	jsonBuf := new(bytes.Buffer)

	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		// Fast decode using vflow
		decoder := ipfix.NewDecoder(raddr.IP, buf[:n])
		msg, err := decoder.Decode(c.ipfixCache)
		if err != nil || msg == nil {
			continue
		}

		if len(msg.DataSets) > 0 {
			jsonBuf.Reset()
			b, _ := msg.JSONMarshal(jsonBuf)

			if c.db != nil {
				c.db.WriteFlows([]store.FlowRecord{{
					HostKey:     raddr.IP.String(),
					HostName:    raddr.IP.String(),
					HostAddress: raddr.IP.String(),
					FlowType:    "ipfix",
					Payload:     b,
					CollectedAt: time.Now(),
				}})
			} else {
				log.Printf("[IPFIX] (No DB) %s", string(b))
			}
		}
	}
}

func (c *IPFlowCollector) listenNetFlow(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: c.NetFlowPort})
	if err != nil {
		log.Fatalf("NetFlow Listen Error: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for NetFlow v9 on UDP :%d", c.NetFlowPort)
	buf := make([]byte, 65535)
	jsonBuf := new(bytes.Buffer)

	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		decoder := netflow9.NewDecoder(raddr.IP, buf[:n])
		msg, err := decoder.Decode(c.netflowCache)
		if err != nil || msg == nil {
			continue
		}

		if len(msg.DataSets) > 0 {
			jsonBuf.Reset()
			b, _ := msg.JSONMarshal(jsonBuf)

			if c.db != nil {
				c.db.WriteFlows([]store.FlowRecord{{
					HostKey:     raddr.IP.String(),
					HostName:    raddr.IP.String(),
					HostAddress: raddr.IP.String(),
					FlowType:    "netflow9",
					Payload:     b,
					CollectedAt: time.Now(),
				}})
			} else {
				log.Printf("[NetFlow] (No DB) %s", string(b))
			}
		}
	}
}

func (c *IPFlowCollector) listenSFlow(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: c.SFlowPort})
	if err != nil {
		log.Fatalf("sFlow Listen Error: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for sFlow on UDP :%d", c.SFlowPort)
	buf := make([]byte, 65535)

	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		reader := bytes.NewReader(buf[:n])
		decoder := sflow.NewSFDecoder(reader, nil)
		datagram, err := decoder.SFDecode()
		if err != nil || datagram == nil {
			continue
		}

		// sFlow records
		if len(datagram.Samples) > 0 {
			b, _ := json.Marshal(datagram)

			if c.db != nil {
				c.db.WriteFlows([]store.FlowRecord{{
					// We capture the sender IP (switch/router), not the internal sampled hosts here
					HostKey:     raddr.IP.String(),
					HostName:    raddr.IP.String(),
					HostAddress: raddr.IP.String(),
					FlowType:    "sflow",
					Payload:     b,
					CollectedAt: time.Now(),
				}})
			} else {
				log.Printf("[sFlow] (No DB) %s", string(b))
			}
		}
	}
}
