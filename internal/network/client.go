package network

import (
	"bufio"
	"net"
	"strings"
	"time"
)

type Client struct {
	conn    net.Conn
	reader  *bufio.Reader
	timeout time.Duration
}

func NewClient(conn net.Conn, timeout time.Duration) *Client {
	return &Client{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		timeout: timeout,
	}
}

func Dial(addr string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, timeout), nil
}

func (c *Client) Send(request string) (string, error) {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return "", err
	}
	if _, err := c.conn.Write([]byte(request + "\n")); err != nil {
		return "", err
	}
	response, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(response, "\r\n"), nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
