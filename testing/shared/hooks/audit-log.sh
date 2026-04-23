#\!/bin/bash
# PostToolUse hook — logs all tool invocations to results/audit.log
# Reads tool result JSON from stdin

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // "unknown"')
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

case "$TOOL_NAME" in
  Bash)
    DETAIL=$(echo "$INPUT" | jq -r '.tool_input.command // ""' | head -c 120)
    ;;
  Edit|Write)
    DETAIL=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // ""')
    ;;
  *)
    DETAIL=$(echo "$INPUT" | jq -r '.tool_input | keys | join(", ")' 2>/dev/null || echo "")
    ;;
esac

mkdir -p results

echo "[$TIMESTAMP] $TOOL_NAME: $DETAIL" >> results/audit.log

exit 0
