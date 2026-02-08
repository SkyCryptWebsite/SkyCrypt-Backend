#!/usr/bin/env bash

# NOTE: This script was vibe coded in a single session to quickly analyze logs without needing a full dashboard. It's not meant to be perfect or handle every edge case, but it provides a useful overview of the logs in a human-friendly format.

# ============================================================
#  SkyCrypt Forensic Log Analyzer
#  Parses JSON logs from logs/app.log and generates a report.
#
#  Usage:
#    ./tools/analyze-logs.sh                  # analyze logs/app.log
#    ./tools/analyze-logs.sh logs/app.log     # analyze specific file
#    ./tools/analyze-logs.sh --live           # tail + analyze in real-time
#
#  Requires: jq
# ============================================================
# NOTE: no `set -e` or `pipefail` — jq returns non-zero on non-JSON lines,
# and the log file may contain plain text mixed with JSON.
set -u

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

LOG_FILE="${1:-logs/app.log}"
ERROR_LOG="logs/error.log"

# Helper: extract only valid JSON lines, then pipe to jq
jqf() {
    grep -a '^\s*{' "$2" 2>/dev/null | jq -r "$1" 2>/dev/null || true
}

if [[ "$LOG_FILE" == "--live" ]]; then
    echo -e "${CYAN}Live forensic tail (Ctrl+C to stop)...${NC}"
    tail -f logs/app.log | grep --line-buffered '^\s*{' | jq -r '
      if .level == "error" then "\u001b[31m[\(.level)] \(.msg) \(.error // "")\u001b[0m"
      elif .level == "warn" then "\u001b[33m[\(.level)] \(.msg) \(.operation // .key // "")\u001b[0m"
      elif .msg == "request_completed" then "[\(.level)] \(.method) \(.path) → \(.status_code) in \(.duration_ms)ms"
      elif .msg == "runtime_stats" then "\u001b[36m[runtime] goroutines=\(.num_goroutines) heap=\(.heap_alloc_mb)MB gc=\(.num_gc)\u001b[0m"
      else empty
      end
    ' 2>/dev/null || tail -f logs/app.log
    exit 0
fi

if ! command -v jq &>/dev/null; then
    echo -e "${RED}Error: jq is required. Install with: sudo pacman -S jq${NC}"
    exit 1
fi

if [[ ! -f "$LOG_FILE" ]]; then
    echo -e "${RED}Error: Log file not found: $LOG_FILE${NC}"
    echo "Run the app first, then try again."
    exit 1
fi

TOTAL_LINES=$(wc -l < "$LOG_FILE")
JSON_LINES=$(grep -ac '^\s*{' "$LOG_FILE" 2>/dev/null || echo 0)
echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║           SKYCRYPT FORENSIC LOG ANALYSIS REPORT            ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  App log:     ${CYAN}$LOG_FILE${NC} ($(du -h "$LOG_FILE" | cut -f1))"
echo -e "  Total lines: ${CYAN}$TOTAL_LINES${NC}  (JSON: ${CYAN}$JSON_LINES${NC})"
if [[ -f "$ERROR_LOG" ]]; then
    ERR_LINES=$(wc -l < "$ERROR_LOG")
    ERR_JSON=$(grep -ac '^\s*{' "$ERROR_LOG" 2>/dev/null || echo 0)
    echo -e "  Error log:   ${CYAN}$ERROR_LOG${NC} ($(du -h "$ERROR_LOG" | cut -f1), ${ERR_JSON} JSON lines)"
else
    echo -e "  Error log:   ${CYAN}(not found)${NC}"
fi
echo ""

# ── Log Level Distribution ──────────────────────────────────
echo -e "${BOLD}━━━ Log Level Distribution ━━━${NC}"
jqf '.level' "$LOG_FILE" | sort | uniq -c | sort -rn | while read -r count level; do
    case "$level" in
        error) color="$RED" ;;
        warn)  color="$YELLOW" ;;
        info)  color="$GREEN" ;;
        debug) color="$CYAN" ;;
        *)     color="$NC" ;;
    esac
    printf "  ${color}%-8s${NC} %s\n" "$level" "$count"
done
echo ""

# ── Top Message Types ───────────────────────────────────────
echo -e "${BOLD}━━━ Top 15 Message Types ━━━${NC}"
jqf '.msg' "$LOG_FILE" | sort | uniq -c | sort -rn | head -15 | while read -r count msg; do
    printf "  %-6s %s\n" "$count" "$msg"
done
echo ""

# ── Error Log Analysis ─────────────────────────────────────
echo -e "${BOLD}━━━ Error Log (logs/error.log) ━━━${NC}"
if [[ -f "$ERROR_LOG" ]]; then
    ERR_TOTAL=$(grep -ac '^\s*{' "$ERROR_LOG" 2>/dev/null || echo 0)
    if [[ "$ERR_TOTAL" -eq 0 ]]; then
        echo -e "  ${GREEN}Error log is empty — no warnings or errors!${NC}"
    else
        echo -e "  Total entries: ${RED}$ERR_TOTAL${NC}"
        echo ""
        echo "  By level:"
        jqf '.level' "$ERROR_LOG" | sort | uniq -c | sort -rn | while read -r count level; do
            case "$level" in
                error|fatal) color="$RED" ;;
                warn)  color="$YELLOW" ;;
                *)     color="$NC" ;;
            esac
            printf "    ${color}%-8s${NC} %s\n" "$level" "$count"
        done
        echo ""
        echo "  Top error/warn messages:"
        jqf '.msg' "$ERROR_LOG" | sort | uniq -c | sort -rn | head -10 | while read -r count msg; do
            printf "    ${RED}%-6s${NC} %s\n" "$count" "$msg"
        done
        echo ""
        echo "  Recent errors (last 5):"
        grep -a '^\s*{' "$ERROR_LOG" 2>/dev/null | tail -5 | jq -r '"\(.timestamp) [\(.level)] \(.msg) \(.error // .operation // .key // "")"' 2>/dev/null | while read -r line; do
            printf "    ${RED}%s${NC}\n" "$line"
        done
    fi
else
    echo -e "  ${CYAN}No error log file found${NC}"
fi
echo ""

# ── Errors & Warnings (from app.log) ──────────────────────
echo -e "${BOLD}━━━ Errors (app.log) ━━━${NC}"
ERROR_COUNT=$(jqf 'select(.level=="error") | .msg' "$LOG_FILE" | wc -l)
if [[ "$ERROR_COUNT" -eq 0 ]]; then
    echo -e "  ${GREEN}No errors recorded!${NC}"
else
    echo -e "  ${RED}Total errors: $ERROR_COUNT${NC}"
    jqf 'select(.level=="error") | .msg' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count msg; do
        printf "  ${RED}%-6s${NC} %s\n" "$count" "$msg"
    done
fi
echo ""

echo -e "${BOLD}━━━ Warnings (app.log) ━━━${NC}"
WARN_COUNT=$(jqf 'select(.level=="warn") | .msg' "$LOG_FILE" | wc -l)
if [[ "$WARN_COUNT" -eq 0 ]]; then
    echo -e "  ${GREEN}No warnings recorded!${NC}"
else
    echo -e "  ${YELLOW}Total warnings: $WARN_COUNT${NC}"
    jqf 'select(.level=="warn") | .msg' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count msg; do
        printf "  ${YELLOW}%-6s${NC} %s\n" "$count" "$msg"
    done
fi
echo ""

# ── Slow Operations ────────────────────────────────────────
echo -e "${BOLD}━━━ Slow Operations (>100ms) ━━━${NC}"
SLOW=$(jqf 'select(.msg=="slow_operation_detected") | .operation' "$LOG_FILE" | wc -l)
if [[ "$SLOW" -eq 0 ]]; then
    echo -e "  ${GREEN}None detected${NC}"
else
    echo -e "  ${YELLOW}Count: $SLOW${NC}"
    jqf 'select(.msg=="slow_operation_detected") | .operation' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count op; do
        printf "  ${YELLOW}%-6s${NC} %s\n" "$count" "$op"
    done
fi
echo ""

echo -e "${BOLD}━━━ Critical Slow Operations (>1s) ━━━${NC}"
CRIT_SLOW=$(jqf 'select(.msg=="critical_slow_operation") | .operation' "$LOG_FILE" | wc -l)
if [[ "$CRIT_SLOW" -eq 0 ]]; then
    echo -e "  ${GREEN}None detected${NC}"
else
    echo -e "  ${RED}Count: $CRIT_SLOW${NC}"
    jqf 'select(.msg=="critical_slow_operation") | "\(.operation) → \(.duration)"' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count entry; do
        printf "  ${RED}%-6s${NC} %s\n" "$count" "$entry"
    done
fi
echo ""

# ── Slow Redis Operations ──────────────────────────────────
echo -e "${BOLD}━━━ Slow Redis Operations (>5ms) ━━━${NC}"
SLOW_REDIS=$(jqf 'select(.msg=="slow_redis_get" or .msg=="slow_redis_set") | .key' "$LOG_FILE" | wc -l)
if [[ "$SLOW_REDIS" -eq 0 ]]; then
    echo -e "  ${GREEN}None detected${NC}"
else
    echo -e "  ${YELLOW}Count: $SLOW_REDIS${NC}"
    jqf 'select(.msg=="slow_redis_get" or .msg=="slow_redis_set") | .key' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count key; do
        printf "  ${YELLOW}%-6s${NC} %s\n" "$count" "$key"
    done
fi
echo ""

# ── Redis Cache Performance ────────────────────────────────
echo -e "${BOLD}━━━ Redis Cache Performance ━━━${NC}"
HITS=$(jqf 'select(.msg=="redis_cache_hit") | .key' "$LOG_FILE" | wc -l)
MISSES=$(jqf 'select(.msg=="redis_cache_miss") | .key' "$LOG_FILE" | wc -l)
TOTAL=$((HITS + MISSES))
if [[ "$TOTAL" -gt 0 ]]; then
    RATE=$(echo "scale=1; $HITS * 100 / $TOTAL" | bc 2>/dev/null || echo "?")
    echo -e "  Hits:     ${GREEN}$HITS${NC}"
    echo -e "  Misses:   ${RED}$MISSES${NC}"
    echo -e "  Hit Rate: ${CYAN}${RATE}%${NC}"
    echo ""
    echo "  Top keys by access count:"
    jqf 'select(.msg=="redis_cache_hit" or .msg=="redis_cache_miss") | .key' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count key; do
        printf "    %-6s %s\n" "$count" "$key"
    done
else
    echo -e "  ${CYAN}No cache operations recorded yet${NC}"
fi
echo ""

# ── HTTP Request Performance ───────────────────────────────
echo -e "${BOLD}━━━ Request Performance ━━━${NC}"
REQ_COUNT=$(jqf 'select(.msg=="request_completed") | .path' "$LOG_FILE" | wc -l)
if [[ "$REQ_COUNT" -eq 0 ]]; then
    echo -e "  ${CYAN}No requests recorded yet${NC}"
else
    echo -e "  Total requests: ${CYAN}$REQ_COUNT${NC}"
    echo ""
    echo "  Slowest requests (by duration_ms):"
    jqf 'select(.msg=="request_completed") | "\(.duration_ms)ms \(.method) \(.path)"' "$LOG_FILE" | sort -t'm' -k1 -rn | head -10 | while read -r line; do
        printf "    %s\n" "$line"
    done
    echo ""
    echo "  Requests by endpoint:"
    jqf 'select(.msg=="request_completed") | .path' "$LOG_FILE" | sed 's|/[a-f0-9-]\{8,\}||g; s|/[a-f0-9]\{32\}||g' | sort | uniq -c | sort -rn | head -15 | while read -r count path; do
        printf "    %-6s %s\n" "$count" "$path"
    done
    echo ""
    echo "  Average response time per endpoint:"
    jqf 'select(.msg=="request_completed") | "\(.path | gsub("/[a-f0-9-]{8,}"; "") | gsub("/[a-f0-9]{32}"; "")) \(.duration_ms)"' "$LOG_FILE" | \
        awk '{sum[$1]+=$2; count[$1]++} END {for (k in sum) printf "    %-50s avg=%.1fms  n=%d\n", k, sum[k]/count[k], count[k]}' | sort -t= -k2 -rn | head -15
fi
echo ""

# ── HTTP Status Code Distribution ──────────────────────────
echo -e "${BOLD}━━━ HTTP Status Codes ━━━${NC}"
jqf 'select(.msg=="request_completed") | .status_code' "$LOG_FILE" | sort | uniq -c | sort -rn | while read -r count code; do
    case "$code" in
        2*) color="$GREEN" ;;
        3*) color="$CYAN" ;;
        4*) color="$YELLOW" ;;
        5*) color="$RED" ;;
        *)  color="$NC" ;;
    esac
    printf "  ${color}%-6s${NC} %s\n" "$code" "$count"
done
echo ""

# ── Outbound HTTP Calls ────────────────────────────────────
echo -e "${BOLD}━━━ Outbound HTTP Calls ━━━${NC}"
OUT_COUNT=$(jqf 'select(.msg=="http_request_completed") | .url' "$LOG_FILE" | wc -l)
if [[ "$OUT_COUNT" -eq 0 ]]; then
    echo -e "  ${CYAN}No outbound HTTP calls recorded${NC}"
else
    echo -e "  Total: ${CYAN}$OUT_COUNT${NC}"
    echo "  By host:"
    jqf 'select(.msg=="http_request_completed") | .url' "$LOG_FILE" | sed 's|https\?://||; s|/.*||' | sort | uniq -c | sort -rn | head -10 | while read -r count host; do
        printf "    %-6s %s\n" "$count" "$host"
    done
    RATE_LIMITED=$(jqf 'select(.msg=="rate_limit_detected") | .url' "$LOG_FILE" | wc -l)
    if [[ "$RATE_LIMITED" -gt 0 ]]; then
        echo -e "  ${RED}Rate-limited responses (429): $RATE_LIMITED${NC}"
    fi
fi
echo ""

# ── N+1 Query Detection ───────────────────────────────────
echo -e "${BOLD}━━━ N+1 Query Patterns ━━━${NC}"
NPLUS1=$(jqf 'select(.msg=="nplus1_query_detected") | "\(.query) (×\(.execution_count))"' "$LOG_FILE" | wc -l)
if [[ "$NPLUS1" -eq 0 ]]; then
    echo -e "  ${GREEN}No N+1 patterns detected${NC}"
else
    echo -e "  ${RED}Detected patterns: $NPLUS1${NC}"
    jqf 'select(.msg=="nplus1_query_detected") | "\(.query) (×\(.execution_count))"' "$LOG_FILE" | sort | uniq -c | sort -rn | head -10 | while read -r count pattern; do
        printf "  ${RED}%-6s${NC} %s\n" "$count" "$pattern"
    done
fi
echo ""

# ── Critical Bottlenecks ──────────────────────────────────
echo -e "${BOLD}━━━ Critical Bottlenecks (>10% of total time) ━━━${NC}"
BOTTLENECKS=$(jqf 'select(.msg=="critical_bottleneck_detected") | .operation' "$LOG_FILE" | wc -l)
if [[ "$BOTTLENECKS" -eq 0 ]]; then
    echo -e "  ${GREEN}No critical bottlenecks identified yet${NC}"
else
    jqf 'select(.msg=="critical_bottleneck_detected") | "\(.percent_of_total_time)% \(.operation) (\(.call_count) calls)"' "$LOG_FILE" | sort -rn | head -10 | while read -r line; do
        printf "  ${RED}%s${NC}\n" "$line"
    done
fi
echo ""

# ── Runtime Anomalies ─────────────────────────────────────
echo -e "${BOLD}━━━ Runtime Anomalies ━━━${NC}"
LEAKS=$(jqf 'select(.msg=="goroutine_leak_detected" or .msg=="memory_leak_suspected" or .msg=="gc_pause_time_critical") | .msg' "$LOG_FILE" | wc -l)
if [[ "$LEAKS" -eq 0 ]]; then
    echo -e "  ${GREEN}No goroutine leaks, memory leaks, or critical GC pauses${NC}"
else
    jqf 'select(.msg=="goroutine_leak_detected" or .msg=="memory_leak_suspected" or .msg=="gc_pause_time_critical") | .msg' "$LOG_FILE" | sort | uniq -c | sort -rn | while read -r count msg; do
        printf "  ${RED}%-6s${NC} %s\n" "$count" "$msg"
    done
fi
echo ""

# ── Panics ────────────────────────────────────────────────
echo -e "${BOLD}━━━ Panics ━━━${NC}"
PANICS=$(jqf 'select(.msg=="panic_recovered") | .url' "$LOG_FILE" | wc -l)
if [[ "$PANICS" -eq 0 ]]; then
    echo -e "  ${GREEN}No panics recovered${NC}"
else
    echo -e "  ${RED}PANICS DETECTED: $PANICS${NC}"
    jqf 'select(.msg=="panic_recovered") | "  URL: \(.url)\n  Error: \(.error)"' "$LOG_FILE" | head -20
fi
echo ""

# ── Memory Trend (last 10 snapshots) ──────────────────────
echo -e "${BOLD}━━━ Memory Trend (last 10 runtime snapshots) ━━━${NC}"
jqf 'select(.msg=="runtime_stats") | "\(.timestamp) goroutines=\(.num_goroutines) heap=\(.heap_alloc_mb)MB sys=\(.sys_mb)MB gc=\(.num_gc)"' "$LOG_FILE" | tail -10 | while read -r line; do
    printf "  %s\n" "$line"
done
echo ""

echo -e "${BOLD}════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Analysis complete.${NC}"
echo -e "  Live dashboard: ${CYAN}http://localhost:8080/api/forensics/dashboard${NC}"
echo -e "  JSON endpoint:  ${CYAN}http://localhost:8080/api/forensics${NC}"
echo -e "  Live tail:      ${CYAN}./tools/analyze-logs.sh --live${NC}"
echo -e "${BOLD}════════════════════════════════════════════════════════════════${NC}"
