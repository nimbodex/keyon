package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nimbodex/keyon/internal/config"
	"github.com/nimbodex/keyon/internal/network"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "client:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("address", "127.0.0.1:3223", "server address")
	timeout := flag.Duration("timeout", 5*time.Second, "request timeout")
	maxSize := flag.String("max-message-size", "4KB", "max response size")
	flag.Parse()

	bufSize, err := config.ParseSize(*maxSize)
	if err != nil {
		return fmt.Errorf("parse max-message-size: %w", err)
	}

	client, err := network.Dial(*addr, *timeout)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			fmt.Fprintln(os.Stderr, "close:", cerr)
		}
	}()

	fmt.Printf("connected to %s\n", *addr)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, bufSize), bufSize)

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			if serr := scanner.Err(); serr != nil {
				return fmt.Errorf("read stdin: %w", serr)
			}
			fmt.Println()
			return nil
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		resp, err := client.Send(line)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("server closed connection")
			}
			fmt.Println("ERROR:", err)
			continue
		}
		fmt.Println(resp)
	}
}
