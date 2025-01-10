package main

import (
	"github.com/ray-project/rayci/reefagent"
)

func main() {
	a := reefagent.Agent{
		Id:    "123",
		Queue: "test",
	}
	a.Start()
}
