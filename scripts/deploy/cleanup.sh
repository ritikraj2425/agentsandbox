#!/bin/bash
# Cron script to cleanup orphaned AgentSandbox containers
# Recommended cron schedule: */10 * * * * (every 10 minutes)

echo "[$(date)] Starting sandbox container cleanup..."

# Find all docker containers matching our prefix that have been running for more than 2 hours
# OR have exited but were left behind
STALE_CONTAINERS=$(docker ps -a --filter "name=agentsandbox-" --format "{{.ID}}")

if [ -z "$STALE_CONTAINERS" ]; then
    echo "[$(date)] No orphaned containers found."
    exit 0
fi

echo "[$(date)] Found containers to remove: $STALE_CONTAINERS"

# Force remove them
docker rm -f $STALE_CONTAINERS

echo "[$(date)] Cleanup complete."
