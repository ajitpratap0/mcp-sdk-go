#!/bin/bash

# =============================================
# MCP Test Script - Bidirectional Stdio Transport
# =============================================

set -e

# Configuration
SERVER_DIR="examples/simple-server"
CLIENT_DIR="examples/simple-client"
LOG_DIR="logs"
VERBOSE=true
TEST_DURATION=20

# Log files
SERVER_LOG="$LOG_DIR/server.log"
CLIENT_LOG="$LOG_DIR/client.log"

# Output formatting
BOLD="\033[1m"
GREEN="\033[32m"
RED="\033[31m"
BLUE="\033[34m"
RESET="\033[0m"

# Utilities
print_header()  { echo -e "\n${BOLD}${BLUE}=== $1 ===${RESET}"; }
print_success() { echo -e "${GREEN}✓ $1${RESET}"; }
print_error()   { echo -e "${RED}✗ $1${RESET}"; }

# Kill background jobs
cleanup() {
  print_header "CLEANUP"

  [[ -n "$SERVER_PID" ]] && kill $SERVER_PID 2>/dev/null || true
  [[ -n "$CLIENT_PID" ]] && kill $CLIENT_PID 2>/dev/null || true

  rm -f server_in server_out client_in client_out
  print_success "Named pipes cleaned up"

  echo "Logs:"
  echo "  $SERVER_LOG"
  echo "  $CLIENT_LOG"
}
trap cleanup EXIT

# Prepare logs and pipes
print_header "SETUP"
mkdir -p "$LOG_DIR"

# Check write permission for the log directory
if [ ! -w "$LOG_DIR" ]; then
  print_error "No write permission to $LOG_DIR. Trying to fix..."
  chmod u+w "$LOG_DIR" || { print_error "Failed to set write permission on $LOG_DIR"; exit 1; }
fi

rm -f "$SERVER_LOG" "$CLIENT_LOG"
rm -f server_in server_out client_in client_out
mkfifo server_in server_out client_in client_out

# Pipe setup:
# client stdout -> server stdin
# server stdout -> client stdin
ln -sf server_out client_in
ln -sf client_out server_in

# Start server with verbose output to ensure it's working
print_header "STARTING SERVER"
go run $SERVER_DIR/main.go < server_in > server_out 2>> "$SERVER_LOG" &
SERVER_PID=$!
print_success "Server running (PID $SERVER_PID)"

# Start client
print_header "STARTING CLIENT"
go run $CLIENT_DIR/main.go < client_in > client_out 2>> "$CLIENT_LOG" &
CLIENT_PID=$!
print_success "Client running (PID $CLIENT_PID)"

'''
print_header "STARTING SERVER"
go run $SERVER_DIR/main.go < server_in > server_out 2>&1 | tee "$SERVER_LOG" &
SERVER_PID=$!
print_success "Server running (PID $SERVER_PID)"

print_header "STARTING CLIENT"
go run $CLIENT_DIR/main.go < client_in > client_out 2>&1 | tee "$CLIENT_LOG" &
CLIENT_PID=$!
print_success "Client running (PID $CLIENT_PID)"
'''

# Let them communicate
print_header "RUNNING TEST"
sleep "$TEST_DURATION"

# Wait for client and server to finish
# Wait for client and server to finish
wait $SERVER_PID
wait $CLIENT_PID

# Done
print_success "Test completed successfully"
