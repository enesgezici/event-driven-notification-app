#!/bin/bash
set -euo pipefail

# Quick test script for the notification system

echo "🚀 Event-Driven Notification System - Quick Test"
echo "=================================================="
echo ""

BASE_URL="http://localhost:8080"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to run this script"
  exit 1
fi

if ! curl -fsS "$BASE_URL/health" >/dev/null; then
  echo "notification server is not reachable at $BASE_URL"
  exit 1
fi

# Test 1: Health check
echo "1️⃣  Health Check"
curl -fsS "$BASE_URL/health" | jq .
echo ""

# Test 2: Create notifications
echo "2️⃣  Creating Batch Notifications"
RESPONSE=$(curl -fsS -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "recipient": "+905551234567",
        "channel": "sms",
        "content": "SMS Notification Test",
        "priority": "high"
      },
      {
        "recipient": "test@example.com",
        "channel": "email",
        "content": "Email Notification Test",
        "priority": "normal"
      },
      {
        "recipient": "user-device-id",
        "channel": "push",
        "content": "Push Notification Test",
        "priority": "low"
      }
    ]
  }')

BATCH_ID=$(echo "$RESPONSE" | jq -r '.batch_id')
NOTIF_ID=$(echo "$RESPONSE" | jq -r '.notifications[0].id')

echo "Created Batch ID: $BATCH_ID"
echo "First Notification ID: $NOTIF_ID"
echo ""

# Test 3: Wait and check metrics
echo "3️⃣  Waiting for processing..."
sleep 3

echo "📊 System Metrics"
curl -fsS "$BASE_URL/metrics" | jq .
echo ""

# Test 4: Check notification status
echo "4️⃣  Notification Status"
curl -fsS "$BASE_URL/notifications/$NOTIF_ID" | jq .
echo ""

# Test 5: List notifications
echo "5️⃣  List Sent Notifications"
curl -fsS "$BASE_URL/notifications?status=sent&size=1" | jq '.notifications[0]'
echo ""

echo "✅ Test completed!"
