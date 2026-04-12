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
	logger := logger.NewLogger(true)
	engine := engine.NewEngine(logger)
	db := compute.New(engine, logger)
	scanner := bufio.NewScanner(os.Stdin)

	logger.Info("database started")

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				logger.Error("failed to read input", slog.String("error", err.Error()))
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
