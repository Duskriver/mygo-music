//go:build animationaudit

package main

import (
	"log"
	"mygo-music/internal/app"
)

func main() {
	if err := app.RunAnimationAudit(); err != nil {
		log.Fatal(err)
	}
}
