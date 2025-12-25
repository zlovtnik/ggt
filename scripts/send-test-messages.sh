#!/bin/bash

# Script to send test Kafka messages to raw.users topic

BROKER="localhost:9092"
TOPIC="raw.users"
KAFKA_BIN="/opt/homebrew/bin"
MESSAGES=(
  '{"user_id": "123", "name": "John Doe"}'
  '{"user_id": "456", "name": "Jane Smith"}'
  '{"user_id": "789", "name": "Bob Johnson"}'
)

for msg in "${MESSAGES[@]}"; do
  echo "$msg" | "$KAFKA_BIN"/kafka-console-producer --bootstrap-server "$BROKER" --topic "$TOPIC"
  sleep 1
done

echo "Sent ${#MESSAGES[@]} messages to $TOPIC"