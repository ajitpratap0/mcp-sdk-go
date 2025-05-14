package main

import (
	"io"
	"log"
	"os/exec"
)

func main() {
	// Pipe: client writes to cw -> server reads from cr
	cr, cw := io.Pipe()
	// Pipe: server writes to sw -> client reads from sr
	sr, sw := io.Pipe()

	// Start the server process
	serverCmd := exec.Command("go", "run", "simple-server/main.go")
	serverCmd.Stdin = cr  // server reads from cr (client writes to cw)
	serverCmd.Stdout = sw // server writes to sw (client reads from sr)
	serverCmd.Stderr = log.Writer()

	// Start the client process
	clientCmd := exec.Command("go", "run", "simple-client/main.go")
	clientCmd.Stdin = sr  // client reads from sr (server writes to sw)
	clientCmd.Stdout = cw // client writes to cw (server reads from cr)
	clientCmd.Stderr = log.Writer()

	// Start both processes
	if err := serverCmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	if err := clientCmd.Start(); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	// Wait for both to finish
	if err := clientCmd.Wait(); err != nil {
		log.Printf("Client exited with error: %v", err)
	}
	if err := serverCmd.Wait(); err != nil {
		log.Printf("Server exited with error: %v", err)
	}
}
