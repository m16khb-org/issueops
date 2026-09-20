#!/bin/sh
set -eu

mkdir -p "$HOME/.omo" "$OMO_CODING_AGENT_DIR"
printf '%s\n' "launcher touched HOME" >"$HOME/.omo/launcher-state"
printf '%s\n' "launcher touched agent root" >"$OMO_CODING_AGENT_DIR/launcher-state"

if [ "${1:-}" = "--version" ]; then
  printf '%s\n' "omo 5.0.0-0.beta.22 (fixture)"
  exit 0
fi

session_dir=
target_tool=
model=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --session-dir)
      session_dir=$2
      shift 2
      ;;
    --tools)
      target_tool=$2
      shift 2
      ;;
    --model)
      model=$2
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

episode_root=$(dirname "$session_dir")
cat >"$episode_root/result.json" <<'EOF'
{"fixture_id":"empty_object","call_count":1,"raw_sha256":"44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","canonical_arguments":{},"schema_sha256":"schema-sha","run_token_sha256":"1923cae09dc7b3efd3d3ef85eb12fb7b7e70e32496c359e44d5c24b4fab442c0","classification":"exact_valid","advertised_valid":true,"canonical_valid":true,"diagnostics":[]}
EOF

printf '{"type":"session","id":"session-1"}\n'
printf '{"type":"message_start","message":{"role":"assistant","model":"%s"}}\n' "$model"
printf '{"type":"tool_execution_start","toolCallId":"call-1","toolName":"%s","args":{}}\n' "$target_tool"
printf '{"type":"tool_execution_end","toolCallId":"call-1","toolName":"%s","result":{"content":[{"type":"text","text":"captured"}],"details":{"fixture":true}},"isError":false}\n' "$target_tool"
printf '{"type":"message_end","message":{"role":"assistant","model":"%s"}}\n' "$model"
