# Automation Quickstart

Discover actor with `hn identity show --json`. Save data.actor. Create using `hn issue open --actor ACTOR --operation create-1 --json TITLE`. If stdout is lost, query `hn issue operation --actor ACTOR --operation create-1 --json`; use original state/parents for identical retries. Read all current metadata with `hn issue list --details --limit 50 --json`, following next_cursor with unchanged mode/limit. If active identity changed, repair binding before retrying actor_mismatch; do not silently choose the new signer.
