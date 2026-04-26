package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"

	"github.com/nimbodex/keyon/internal/compute"
	"github.com/nimbodex/keyon/internal/storage/engine"
	"github.com/nimbodex/keyon/pkg/logger"
)

func main() {
	log, closer, err := logger.NewLogger("debug", "stdout")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to init logger:", err)
		os.Exit(1)
	}
	defer func() {
		if err := closer.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to close logger:", err)
		}
	}()

	engine := engine.NewEngine(log)
	db := compute.New(engine, log)
	scanner := bufio.NewScanner(os.Stdin)

	log.Info("database started")

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Error("failed to read input", slog.String("error", err.Error()))
			}
			break
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		res, err := db.Handle(line)
		if err != nil {
			fmt.Println("ERROR:", err)
			continue
		}
		fmt.Println(res)
	}
}
