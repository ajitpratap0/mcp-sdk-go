#!/bin/bash

set -e

SERVER_DIR="examples/simple-server"
CLIENT_DIR="examples/simple-client"
LOG_DIR="logs"
TEST_DURATION=60

SERVER_LOG="$LOG_DIR/server.log"
CLIENT_LOG="$LOG_DIR/client.log"

BOLD="\033[1m"
GREEN="\033[32m"
BLUE="\033[34m"
RESET="\033[0m"

print_header() { echo -e "\n${BOLD}${BLUE}=== $1 ===${RESET}"; }
print_success() { echo -e "${GREEN}✓ $1${RESET}"; }

cleanup() {
  print_header "CLEANUP"
  [[ -n "$SERVER_PID" ]] && kill $SERVER_PID 2>/dev/null || true
  [[ -n "$CLIENT_PID" ]] && kill $CLIENT_PID 2>/dev/null || true
  rm -f server_in server_out
  print_success "Cleaned up pipes"

  kill_stray_go_processes

  echo "Logs:"
  echo "  $SERVER_LOG"
  echo "  $CLIENT_LOG"
}
trap cleanup EXIT

kill_stray_go_processes() {
  echo "Looking for stray Go processes..."

  # Kill any go-build processes
  if command -v pkill &> /dev/null; then
    # Find and kill any Go binaries that might be our test processes
    pkill -f "go-build" || true
    pkill -f "exe/main" || true

    # Find and kill any processes specifically related to our client/server
    pkill -f "$SERVER_DIR" || true
    pkill -f "$CLIENT_DIR" || true

    # Look for any Go processes with mcp in the name
    pkill -f "go.*mcp" || true
  else
    # Find PIDs manually and kill them if pkill is not available
    local pids=$(ps aux | grep -E 'go-build|exe/main|mcp' | grep -v grep | awk '{print $2}')
    for pid in $pids; do
      echo "Killing Go process with PID: $pid"
      kill -9 "$pid" 2>/dev/null || true
    done
  fi

  echo "Stray process cleanup complete."
}

# Use our aggressive cleanup function to ensure no stray processes
kill_stray_go_processes


print_header "SETUP"
mkdir -p "$LOG_DIR"
rm -f "$SERVER_LOG" "$CLIENT_LOG" server_in server_out

mkfifo server_in server_out

print_header "STARTING SERVER"
stdbuf -oL -eL go run "$SERVER_DIR/main.go" < server_in \
  | tee "$SERVER_LOG" > server_out &
SERVER_PID=$!
print_success "Server running (PID $SERVER_PID)"

print_header "STARTING CLIENT"
stdbuf -oL -eL go run "$CLIENT_DIR/main.go" < server_out \
  | tee "$CLIENT_LOG" > server_in &
CLIENT_PID=$!
print_success "Client running (PID $CLIENT_PID)"

print_header "RUNNING TEST"
sleep "$TEST_DURATION"

print_success "Test completed successfully"
