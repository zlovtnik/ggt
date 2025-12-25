#!/bin/bash

# Script to send test Kafka messages to raw.users topic

set -e   # Exit on any error
set -o pipefail  # Catch failures in pipes

BROKER="localhost:9092"
TOPIC="raw.users"

# Find kafka-console-producer
KAFKA_PRODUCER=""
if command -v kafka-console-producer >/dev/null 2>&1; then
    KAFKA_PRODUCER="kafka-console-producer"
elif [ -x "/opt/homebrew/bin/kafka-console-producer" ]; then
    KAFKA_PRODUCER="/opt/homebrew/bin/kafka-console-producer"
elif [ -x "/usr/local/bin/kafka-console-producer" ]; then
    KAFKA_PRODUCER="/usr/local/bin/kafka-console-producer"
else
    echo "Error: kafka-console-producer not found in PATH or common locations" >&2
    exit 1
fi

MESSAGES=(
  '{"user_id": "123", "name": "John Doe"}'
  '{"user_id": "456", "name": "Jane Smith"}'
  '{"user_id": "789", "name": "Bob Johnson"}'
)

for msg in "${MESSAGES[@]}"; do
  if ! echo "$msg" | "$KAFKA_PRODUCER" --bootstrap-server "$BROKER" --topic "$TOPIC"; then
    echo "Failed to send message: $msg to $TOPIC on $BROKER" >&2
    exit 1
  fi
  sleep 1
done

echo "Sent ${#MESSAGES[@]} messages to $TOPIC"