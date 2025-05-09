// Package main demonstrates how to use the MCP SDK to create a client that
// connects to an MCP server. It shows how to check the server's capabilities
// and use various features, like tools, resources, and prompts.
//
// This example creates a client that connects to a server via stdio transport,
// checks the server's capabilities, and then demonstrates how to use several
// key features of the MCP protocol:
//
// 1. Tools: Lists all available tools and calls the "hello" tool if present
// 2. Resources: Lists resources and templates, and reads a resource if available
// 3. Prompts: Lists prompts and gets detailed information for one prompt
//
// To run this example, you need an MCP server. You can use one of the server
// examples from this package or any compatible MCP server.
//
// Usage:
//
//	cd examples/simple-client
//	go run main.go
//
// The client will connect to the server via stdio, which means it can be used
// in a pipeline with a server:
//
//	go run examples/simple-server/main.go | go run examples/simple-client/main.go
package main
