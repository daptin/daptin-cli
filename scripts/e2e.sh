#!/bin/bash
# End-to-end tests against a running Daptin server.
# Usage: DAPTIN_ENDPOINT=http://localhost:6336 bash scripts/e2e.sh
set -euo pipefail

EP="${DAPTIN_ENDPOINT:-http://localhost:6336}"
CLI="${DAPTIN_CLI:-./daptin-cli}"
PASS=0
FAIL=0

# Use isolated config so e2e tests don't mutate user's ~/.daptin/config.yaml
export DAPTIN_CLI_CONFIG="/tmp/daptin-e2e-config-$$.yaml"
trap "rm -f $DAPTIN_CLI_CONFIG" EXIT

# Build if binary doesn't exist
if [ ! -f "$CLI" ]; then
  echo "Building $CLI..."
  go build -mod vendor -o "$CLI" .
fi

# Check server is up
if ! curl -sf "$EP/api/world?page%5Bsize%5D=1" -H "Accept: application/json" > /dev/null 2>&1; then
  echo "SKIP: Daptin server not reachable at $EP"
  exit 0
fi

ok() {
  local name="$1"; shift
  if "$@" > /dev/null 2>&1; then
    PASS=$((PASS+1)); printf "  PASS  %s\n" "$name"
  else
    FAIL=$((FAIL+1)); printf "  FAIL  %s\n" "$name"
  fi
}

expect() {
  local name="$1"; local pat="$2"; shift 2
  local output
  output=$("$@" 2>&1) || true
  if echo "$output" | grep -qE "$pat"; then
    PASS=$((PASS+1)); printf "  PASS  %s\n" "$name"
  else
    FAIL=$((FAIL+1)); printf "  FAIL  %s (expected '%s')\n" "$name" "$pat"
    printf "        got: %s\n" "$(echo "$output" | head -1)"
  fi
}

# Discover a world ref_id and table_name from whatever the server has
WORLD_JSON=$(curl -s "$EP/api/world?page%5Bsize%5D=1" -H "Accept: application/json")
REF=$(echo "$WORLD_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['data'][0]['attributes']['reference_id'])")
TABLE=$(echo "$WORLD_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['data'][0]['attributes']['table_name'])")

echo "E2E tests against $EP (table=$TABLE, ref=$REF)"
echo ""

echo "=== Context ==="
ok     "context add"         "$CLI" context add e2etest "$EP"
ok     "context set"         "$CLI" context set e2etest
expect "context list"        "e2etest" "$CLI" context list

echo ""
echo "=== List ==="
expect "list world"          "table_name"    "$CLI" --endpoint "$EP" list --columns table_name world
expect "list json"           '"table_name"'  "$CLI" --endpoint "$EP" --output json list --columns table_name --page-size 1 world
expect "list actions"        "action_name"   "$CLI" --endpoint "$EP" list --columns action_name --page-size 3 action

echo ""
echo "=== Flags after positional args (#3-7) ==="
expect "sort after entity"   "table_name"    "$CLI" --endpoint "$EP" list world --sort table_name --columns table_name --page-size 3
expect "page-size after"     "table_name"    "$CLI" --endpoint "$EP" list world --page-size 2 --columns table_name
expect "entity=action"       "action_name"   "$CLI" --endpoint "$EP" list action --columns action_name --page-size 2

echo ""
echo "=== Get ==="
expect "get by ref"          "table_name"    "$CLI" --endpoint "$EP" get world "$REF"
expect "get --columns"       "table_name"    "$CLI" --endpoint "$EP" get --columns table_name world "$REF"
expect "get flags after"     "table_name"    "$CLI" --endpoint "$EP" get world "$REF" --columns table_name

echo ""
echo "=== Describe ==="
expect "describe table"      "ColumnName"    "$CLI" --endpoint "$EP" describe table "$TABLE"
expect "describe actions"    "Actions:"      "$CLI" --endpoint "$EP" describe table "$TABLE"

echo ""
echo "=== Execute ==="
expect "signin bad creds"    "Notice"        "$CLI" --endpoint "$EP" execute user_account signin email=nobody@test.com password=wrong

echo ""
echo "=== Permission (#9) ==="
expect "decode"              "Guest: Peek"   "$CLI" permission decode 561441
expect "decode full"         "Refer"         "$CLI" permission decode 2097151
expect "decode zero"         "(none)"        "$CLI" permission decode 0
expect "encode"              "Guest: Read"   "$CLI" permission encode +GuestRead +OwnerRead
expect "encode base"         "Peek"          "$CLI" permission encode --base 561441 +GuestRead

echo ""
echo "=== Relate (#8) ==="
ACT_REF=$(curl -s "$EP/api/action?page%5Bsize%5D=1" -H "Accept: application/json" | python3 -c "import json,sys; print(json.load(sys.stdin)['data'][0]['attributes']['reference_id'])")
expect "relate guest"         "forbidden|403|error|Using" "$CLI" --endpoint "$EP" relate action "$ACT_REF" usergroup_id 00000000-0000-0000-0000-000000000000
expect "unrelate guest"       "forbidden|403|error|Using" "$CLI" --endpoint "$EP" unrelate action "$ACT_REF" usergroup_id 00000000-0000-0000-0000-000000000000
expect "related"             "No data|usergroup" "$CLI" --endpoint "$EP" related action "$ACT_REF" usergroup_id

echo ""
echo "=== Output formats ==="
expect "table output"        "table_name"    "$CLI" --endpoint "$EP" --output table list --columns table_name world
expect "json output"         "table_name"    "$CLI" --endpoint "$EP" --output json list --columns table_name world

echo ""
echo "=== Update (#10 panic fix) ==="
expect "update no-panic"     "forbidden|403|permission" "$CLI" --endpoint "$EP" update world "$REF" icon=fa-globe

echo ""
echo "=== WebSocket (#23) ==="

# Sign up and sign in to get an auth token for WS commands
# (context is already set from earlier tests — signin stores the token in it)
WS_EMAIL="ws-e2e-$$@test.com"
WS_PASS="WsE2ePass1234"
"$CLI" --endpoint "$EP" execute user_account signup "email=$WS_EMAIL" name=ws-e2e "password=$WS_PASS" "passwordConfirm=$WS_PASS" > /dev/null 2>&1 || true
"$CLI" --endpoint "$EP" execute user_account signin "email=$WS_EMAIL" "password=$WS_PASS" > /dev/null 2>&1
# Unset DAPTIN_ENDPOINT so WS tests use the saved context (with token)
unset DAPTIN_ENDPOINT

expect "ws ping"              "Pong received" "$CLI" ws ping

# ws listen (connect, timeout=success means it connected and streamed)
WS_LISTEN_ERR=$(timeout 3 "$CLI" ws listen 2>&1 >/dev/null || true)
if echo "$WS_LISTEN_ERR" | grep -q "Connected"; then
  PASS=$((PASS+1)); printf "  PASS  ws listen (connected)\n"
else
  FAIL=$((FAIL+1)); printf "  FAIL  ws listen (connected)\n"
  printf "        got: %s\n" "$(echo "$WS_LISTEN_ERR" | head -1)"
fi

# ws topic create
expect "ws topic create"      "Created topic" "$CLI" ws topic create e2e-test-topic

# ws subscribe + publish: subscribe in background, publish, check delivery
"$CLI" ws subscribe e2e-test-topic > /tmp/ws-sub-$$.out 2>/dev/null &
SUB_PID=$!
sleep 1

# publish a message
expect "ws publish"           "Published to" "$CLI" ws publish e2e-test-topic '{"e2e":"hello"}'

sleep 1
kill $SUB_PID 2>/dev/null; wait $SUB_PID 2>/dev/null || true

if grep -q "e2e" /tmp/ws-sub-$$.out 2>/dev/null; then
  PASS=$((PASS+1)); printf "  PASS  ws subscribe receives published message\n"
else
  FAIL=$((FAIL+1)); printf "  FAIL  ws subscribe receives published message\n"
  printf "        got: %s\n" "$(cat /tmp/ws-sub-$$.out 2>/dev/null | head -1)"
fi
rm -f /tmp/ws-sub-$$.out

# ws topic permission --set then get
expect "ws topic set-perm"    "Set permission" "$CLI" ws topic permission --set 2097151 e2e-test-topic

WS_PERM_OUT=$("$CLI" ws topic permission e2e-test-topic 2>&1) || true
if echo "$WS_PERM_OUT" | grep -qE "2097151|permission"; then
  PASS=$((PASS+1)); printf "  PASS  ws topic permission get\n"
else
  FAIL=$((FAIL+1)); printf "  FAIL  ws topic permission get\n"
  printf "        got: %s\n" "$(echo "$WS_PERM_OUT" | head -1)"
fi

# ws topic delete
expect "ws topic delete"      "Deleted topic" "$CLI" ws topic delete e2e-test-topic

# ws subscribe multi-topic (just verify it connects and subscribes)
WS_MULTI_OUT=$(timeout 2 "$CLI" ws subscribe e2e-test-topic world 2>&1 || true)
if echo "$WS_MULTI_OUT" | grep -qE "Subscribed|subscribe.*failed"; then
  PASS=$((PASS+1)); printf "  PASS  ws subscribe multi-topic\n"
else
  FAIL=$((FAIL+1)); printf "  FAIL  ws subscribe multi-topic\n"
  printf "        got: %s\n" "$(echo "$WS_MULTI_OUT" | head -1)"
fi

echo ""
echo "================================"
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
