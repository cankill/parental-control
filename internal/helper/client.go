package helper

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client отправляет команды блокировки helper-демону через Unix domain socket.
type Client struct {
	sockPath string
}

func NewClient() *Client {
	return &Client{sockPath: SockPath}
}

// BlockDomains просит helper заблокировать домены в /etc/hosts.
func (c *Client) BlockDomains(domains []string) error {
	return c.send(Request{Command: CommandBlock, Domains: domains})
}

// UnblockDomains просит helper разблокировать домены.
func (c *Client) UnblockDomains(domains []string) error {
	return c.send(Request{Command: CommandUnblock, Domains: domains})
}

func (c *Client) send(req Request) error {
	conn, err := net.DialTimeout("unix", c.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect helper: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("helper error: %s", resp.Error)
	}
	return nil
}
