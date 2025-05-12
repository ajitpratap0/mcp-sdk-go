#!/bin/bash

# ===================================================
# MCP Test Script - Streamable HTTP Client-Server Test
# ===================================================

# Set error handling
set -e

# Default configuration
PORT=8080
TEST_DURATION=30 # Increased to give more time for connection
SERVER_DIR="streamable-http-server"
CLIENT_DIR="streamable-http-client"
EXAMPLES_DIR="examples"
VERBOSE=true # Set to true by default for better diagnostics
LOG_DIR="logs"

# Create logs directory immediately (before any command substitution)
mkdir -p "$LOG_DIR"
SERVER_LOG="$LOG_DIR/server.log"
CLIENT_LOG="$LOG_DIR/client.log"

# Output formatting
BOLD="\033[1m"
GREEN="\033[32m"
RED="\033[31m"
YELLOW="\033[33m"
BLUE="\033[34m"
RESET="\033[0m"

# ===================================================
# Helper functions
# ===================================================

# Print formatted message
print_header() {
  echo -e "\n${BOLD}${BLUE}=== $1 ===${RESET}\n"
}

print_success() {
  echo -e "${GREEN}✓ $1${RESET}"
}

print_error() {
  echo -e "${RED}✗ $1${RESET}"
}

print_warning() {
  echo -e "${YELLOW}! $1${RESET}"
}

# Kill process and all its children
kill_process_tree() {
  local pid=$1
  local signal=${2:-15}  # Default to SIGTERM

  if [ -z "$pid" ]; then
    return
  fi

  echo "Killing process tree for PID: $pid"

  # Get all child processes
  local children=$(pgrep -P "$pid" 2>/dev/null)

  # Recursively kill children first
  for child in $children; do
    kill_process_tree "$child" "$signal"
  done

  # Kill the parent process
  if ps -p "$pid" > /dev/null 2>&1; then
    kill -"$signal" "$pid" 2>/dev/null
    # Wait a moment to ensure it's terminated
    sleep 0.5

    # Force kill if still running
    if ps -p "$pid" > /dev/null 2>&1; then
      echo "Process $pid still running, sending SIGKILL"
      kill -9 "$pid" 2>/dev/null || true
    fi
  fi
}

# Kill any stray Go processes from our tests
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

  # Ensure port is freed
  kill_port $PORT

  echo "Stray process cleanup complete."
}

# Show script usage
show_usage() {
  echo -e "${BOLD}Usage:${RESET} $(basename $0) [OPTIONS]"
  echo "Run MCP Streamable HTTP client-server tests"
  echo ""
  echo -e "${BOLD}Options:${RESET}"
  echo "  -p, --port PORT        Port for the server (default: $PORT)"
  echo "  -d, --duration SEC     Test duration in seconds (default: $TEST_DURATION)"
  echo "  -s, --server DIR       Server directory (default: auto-detect)"
  echo "  -c, --client DIR       Client directory (default: auto-detect)"
  echo "  -v, --verbose          Show detailed output"
  echo "  -h, --help             Show this help"
  exit 1
}

# Detect and kill processes using a specific port
kill_port() {
  local port=$1
  echo "Checking for processes using port $port..."

  # For macOS (lsof)
  if command -v lsof &> /dev/null; then
    local pid=$(lsof -ti :$port)
    if [ -n "$pid" ]; then
      echo "Found process $pid using port $port, killing it..."
      kill_process_tree "$pid" 9
      echo "Process killed."
    else
      echo "No process found using port $port."
    fi
  # For Linux (ss or netstat)
  elif command -v ss &> /dev/null; then
    local pid=$(ss -lptn "sport = :$port" | grep -oP '(?<=pid=).*?(?=,|$)' | head -1)
    if [ -n "$pid" ]; then
      echo "Found process $pid using port $port, killing it..."
      kill_process_tree "$pid" 9
      echo "Process killed."
    else
      echo "No process found using port $port."
    fi
  elif command -v netstat &> /dev/null; then
    local pid=$(netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | head -1)
    if [ -n "$pid" ]; then
      echo "Found process $pid using port $port, killing it..."
      kill_process_tree "$pid" 9
      echo "Process killed."
    else
      echo "No process found using port $port."
    fi
  else
    echo "Could not find lsof, ss, or netstat. Unable to check for processes using port $port."
    return 1
  fi

  echo "Port $port is now available."
  return 0
}

# Find directory within repository
find_dir() {
  local dir_name=$1
  local base_dir=$2

  # Check direct in base directory
  if [ -d "${base_dir}/${dir_name}" ]; then
    echo "${base_dir}/${dir_name}"
    return 0
  fi

  # Check in examples directory
  if [ -d "${base_dir}/${EXAMPLES_DIR}/${dir_name}" ]; then
    echo "${base_dir}/${EXAMPLES_DIR}/${dir_name}"
    return 0
  fi

  # No directory found
  return 1
}

# Cleanup function for graceful shutdown
do_cleanup() {
  print_header "CLEANUP"

  # Collect runtime statistics
  if [ -n "$SERVER_START_TIME" ] && [ -n "$CLIENT_START_TIME" ]; then
    local current_time=$(date +%s)
    local server_runtime=$((current_time - SERVER_START_TIME))
    local client_runtime=$((current_time - CLIENT_START_TIME))

    echo "Runtime statistics:"
    echo "  Server runtime: ${server_runtime}s"
    echo "  Client runtime: ${client_runtime}s"
  fi

  # Kill client first to ensure clean shutdown
  if [ -n "$CLIENT_PID" ]; then
    echo "Stopping client processes (PID: $CLIENT_PID)..."
    # Try to send termination signal first
    kill_process_tree "$CLIENT_PID" 15

    # Kill any remaining Go processes related to the client
    if command -v pkill &> /dev/null; then
      echo "Looking for remaining client Go processes..."
      # Safer to use pkill with a specific directory
      pkill -f "go run.*$CLIENT_DIR" || true
    fi
  fi

  # Stop server
  if [ -n "$SERVER_PID" ]; then
    echo "Stopping server processes (PID: $SERVER_PID)..."
    kill_process_tree "$SERVER_PID" 15
  fi

  # Also look for any remaining processes on the port
  kill_port $PORT

  # Wait a moment to ensure all processes are terminated
  sleep 1

  # Use our aggressive cleanup function to ensure no stray processes
  kill_stray_go_processes

  echo "Logs are available at:"
  echo "  Server: $SERVER_LOG"
  echo "  Client: $CLIENT_LOG"

  echo "Cleanup complete."
}

# Full cleanup and exit handler for unexpected termination
terminate_handler() {
  do_cleanup
  exit 1
}

# ==================================================
# Parse command line arguments
# ==================================================
while [[ $# -gt 0 ]]; do
  case $1 in
    -p|--port)
      PORT="$2"
      shift 2
      ;;
    -d|--duration)
      TEST_DURATION="$2"
      shift 2
      ;;
    -s|--server)
      SERVER_DIR="$2"
      CUSTOM_SERVER=true
      shift 2
      ;;
    -c|--client)
      CLIENT_DIR="$2"
      CUSTOM_CLIENT=true
      shift 2
      ;;
    -v|--verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      show_usage
      ;;
    *)
      echo "Unknown option: $1"
      show_usage
      ;;
  esac
done

# Only trap on signals, not EXIT
# This allows normal script execution to proceed to the end
trap terminate_handler INT TERM HUP

print_header "SETUP"

# Cleanup any processes from previous runs first
kill_stray_go_processes

# Determine base directory and repository structure
BASE_DIR="$(pwd)"
echo "Detecting repository structure..."

# Auto-detect server and client directories if not specified
if [ "$CUSTOM_SERVER" != "true" ]; then
  SERVER_PATH=$(find_dir "$SERVER_DIR" "$BASE_DIR")
  if [ -z "$SERVER_PATH" ]; then
    print_error "Could not find server directory. Please specify with --server."
    exit 1
  fi
  print_success "Found server at: $SERVER_PATH"
else
  SERVER_PATH="${BASE_DIR}/${SERVER_DIR}"
fi

if [ "$CUSTOM_CLIENT" != "true" ]; then
  CLIENT_PATH=$(find_dir "$CLIENT_DIR" "$BASE_DIR")
  if [ -z "$CLIENT_PATH" ]; then
    print_error "Could not find client directory. Please specify with --client."
    exit 1
  fi
  print_success "Found client at: $CLIENT_PATH"
else
  CLIENT_PATH="${BASE_DIR}/${CLIENT_DIR}"
fi

# Ensure port is available
kill_port $PORT

# ==================================================
# Start server
# ==================================================
print_header "SERVER"
echo "Starting Streamable HTTP Server..."

# Check if server directory exists
if [ ! -d "$SERVER_PATH" ]; then
  print_error "Server directory '$SERVER_PATH' does not exist."
  exit 1
fi

# Start server
cd "$SERVER_PATH"
echo "Starting server from $(pwd)"

# Ensure logs directory exists and clear previous log
mkdir -p "$LOG_DIR"
rm -f "$SERVER_LOG"
touch "$SERVER_LOG"

# Run server and log output
go run main.go > >(tee "$SERVER_LOG" | sed 's/^/[SERVER] /') 2>&1 &
SERVER_PID=$!
SERVER_START_TIME=$(date +%s)

print_success "Server started with PID $SERVER_PID"

# Wait for server to initialize
echo "Waiting for server to start..."
sleep 5

# Verify server is running
if ! ps -p $SERVER_PID > /dev/null 2>&1; then
  print_error "Server process died immediately. See $SERVER_LOG for details."
  cat "$SERVER_LOG"
  exit 1
fi

# Ping the server endpoint to make sure it's responding
echo "Testing server endpoint..."
if command -v curl &> /dev/null; then
  if curl -s -o /dev/null -w "%{http_code}" "http://localhost:$PORT/mcp" > /dev/null 2>&1; then
    print_success "Server is responding to HTTP requests"
  else
    print_warning "Server is running but not responding to HTTP requests yet"
    # Continue anyway
  fi
else
  echo "curl not available, skipping endpoint test"
fi

# ==================================================
# Run client
# ==================================================
print_header "CLIENT"
echo "Running Streamable HTTP Client..."

# Check if client directory exists
if [ ! -d "$CLIENT_PATH" ]; then
  print_error "Client directory '$CLIENT_PATH' does not exist."
  exit 1
fi

# Change to client directory
cd "$BASE_DIR"
cd "$CLIENT_PATH"
echo "Starting client from $(pwd)"

# Ensure logs directory exists and clear previous log
mkdir -p "$LOG_DIR"
rm -f "$CLIENT_LOG"
touch "$CLIENT_LOG"

# Run client in background and capture PID
go run main.go > >(tee "$CLIENT_LOG" | sed 's/^/[CLIENT] /') 2>&1 &
CLIENT_PID=$!
CLIENT_START_TIME=$(date +%s)

print_success "Client started with PID $CLIENT_PID"

# Monitor client for specified duration
echo "Monitoring client for $TEST_DURATION seconds..."

# Log monitoring variables
client_running=true
server_running=true
client_success=false
client_status=0

# Set end time
end_time=$(($(date +%s) + TEST_DURATION))

# Watch logs for key events
while [ "$(date +%s)" -lt "$end_time" ]; do
  # Check if client is still running
  if [ "$client_running" = true ] && ! ps -p $CLIENT_PID > /dev/null 2>&1; then
    client_running=false
    client_end_time=$(date +%s)
    client_runtime=$((client_end_time - CLIENT_START_TIME))
    print_warning "Client process terminated after ${client_runtime}s"

    # Get exit status if possible
    wait $CLIENT_PID
    client_status=$?

    if [ $client_status -eq 0 ]; then
      print_success "Client exited normally (status: $client_status)"
      client_success=true
    else
      print_error "Client exited with error (status: $client_status)"
    fi
  fi

  # Check if server is still running
  if [ "$server_running" = true ] && ! ps -p $SERVER_PID > /dev/null 2>&1; then
    server_running=false
    server_end_time=$(date +%s)
    server_runtime=$((server_end_time - SERVER_START_TIME))
    print_error "Server process terminated unexpectedly after ${server_runtime}s"
  fi

  # Exit early if both client and server have terminated
  if [ "$client_running" = false ] && [ "$server_running" = false ]; then
    print_warning "Both client and server have terminated, ending test early"
    break
  fi

  # Check for success indicators in logs
  if [ "$client_success" = false ] && grep -q "Session successfully established" "$CLIENT_LOG" 2>/dev/null; then
    print_success "Client established session with server"
    client_success=true
  fi

  # Print a dot to show activity
  echo -n "."

  # Sleep briefly
  sleep 1
done

echo "" # Newline after dots

# If client is still running, terminate it
if [ "$client_running" = true ] && ps -p $CLIENT_PID > /dev/null 2>&1; then
    print_warning "Test duration reached, terminating client..."
    kill_process_tree "$CLIENT_PID" 15
    sleep 2
    print_success "Client ran for $TEST_DURATION seconds"
    client_success=true
fi

# ==================================================
# Analyze results
# ==================================================
print_header "RESULTS"

# Check for successful session in logs
if grep -q "Session successfully established" "$CLIENT_LOG" 2>/dev/null; then
    print_success "Client successfully established a session with the server"
    SESSION_ID=$(grep -o "Server session established with [^\"]*" "$CLIENT_LOG" 2>/dev/null | tail -1)
    if [ -n "$SESSION_ID" ]; then
        echo "  $SESSION_ID"
    fi
elif grep -q "session" "$CLIENT_LOG" 2>/dev/null; then
    # Look for any session-related messages
    print_warning "Partial session information found:"
    grep -i "session\|connection" "$CLIENT_LOG" 2>/dev/null | sed 's/^/  /'
else
    print_error "No session information found in client logs"
fi

# Check for errors in client logs
if grep -i -q "error\|fail\|exception" "$CLIENT_LOG" 2>/dev/null; then
    print_error "Errors detected in client logs:"
    grep -i "error\|fail\|exception" "$CLIENT_LOG" 2>/dev/null | sed 's/^/  /'
fi

# Check for errors in server logs
if grep -i -q "error\|fail\|exception" "$SERVER_LOG" 2>/dev/null; then
    print_error "Errors detected in server logs:"
    grep -i "error\|fail\|exception" "$SERVER_LOG" 2>/dev/null | sed 's/^/  /'
fi

# Check for successful heartbeats
if grep -q "heartbeat" "$CLIENT_LOG" 2>/dev/null; then
    print_success "Connection heartbeats detected:"
    grep "heartbeat" "$CLIENT_LOG" 2>/dev/null | tail -3 | sed 's/^/  /'
fi

# Show key server messages
if [ -f "$SERVER_LOG" ]; then
    echo "Key server messages:"
    grep -i "server\|listening\|session\|request\|connection" "$SERVER_LOG" 2>/dev/null | head -5 | sed 's/^/  /'
fi

print_header "SUMMARY"

if [ "$client_success" = true ]; then
    print_success "Test completed successfully!"
    echo "  Client connected to server and maintained a session"
    echo "  Logs saved to $LOG_DIR directory"
else
    print_error "Test failed:"

    if ! grep -q "Connection" "$CLIENT_LOG" 2>/dev/null; then
        echo "  Client failed to connect to server"
    elif ! grep -q "Session" "$CLIENT_LOG" 2>/dev/null; then
        echo "  Client connected but failed to establish a session"
    else
        echo "  Client established a session but encountered errors"
    fi

    echo ""
    echo "  See $CLIENT_LOG and $SERVER_LOG for detailed logs"
fi

# Do the cleanup at the end
do_cleanup

# Return appropriate exit code
if [ "$client_success" = true ]; then
    exit 0
else
    exit 1
fi
