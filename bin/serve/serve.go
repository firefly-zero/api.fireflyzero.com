package main

import (
	"log"

	"github.com/firefly-zero/api.fireflyzero.com/lib"
)

func main() {
	err := lib.Run()
	if err != nil {
		log.Fatalf("Fatal server error: %v", err)
	}
}
