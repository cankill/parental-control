package helper

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)

// HandleConnection читает один Request из соединения, проверяет права отправителя
// и домены, выполняет команду и пишет Response. Соединение одноразовое.
func HandleConnection(conn net.Conn) {
	defer conn.Close()

	if err := authenticatePeer(conn); err != nil {
		log.Printf("rejected peer: %v", err)
		writeResponse(conn, Response{Success: false, Error: "unauthorized"})
		return
	}

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeResponse(conn, Response{Success: false, Error: "invalid request"})
		return
	}

	if err := validateDomains(req.Domains); err != nil {
		log.Printf("rejected domains %v: %v", req.Domains, err)
		writeResponse(conn, Response{Success: false, Error: err.Error()})
		return
	}

	var err error
	switch req.Command {
	case CommandBlock:
		err = blockHosts(req.Domains)
		if err == nil {
			log.Printf("blocked: %v", req.Domains)
		}
	case CommandUnblock:
		err = unblockHosts(req.Domains)
		if err == nil {
			log.Printf("unblocked: %v", req.Domains)
		}
	default:
		writeResponse(conn, Response{Success: false, Error: "unknown command"})
		return
	}

	if err != nil {
		log.Printf("command %q failed: %v", req.Command, err)
		writeResponse(conn, Response{Success: false, Error: err.Error()})
		return
	}
	writeResponse(conn, Response{Success: true})
}

// validateDomains требует непустой список и каждый домен из whitelist.
func validateDomains(domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no domains")
	}
	for _, d := range domains {
		if !AllowedDomains[d] {
			return fmt.Errorf("domain not allowed: %s", d)
		}
	}
	return nil
}

func writeResponse(conn net.Conn, resp Response) {
	_ = json.NewEncoder(conn).Encode(resp)
}
