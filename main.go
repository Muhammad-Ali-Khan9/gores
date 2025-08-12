package main

import (
    "github.com/Muhammad-Ali-Khan9/go-microservice-boilerplate/cmd"
    "log"
)

func main() {
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}