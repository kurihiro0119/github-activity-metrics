#!/usr/bin/env python3
"""
Generate test CSV data for events table
"""
import csv
import json
import random
from datetime import datetime, timedelta
from uuid import uuid4

# Sample data
ORGS = ["acme-corp", "tech-startup", "open-source-org"]
USERS = ["alice", "bob", "charlie", "diana", "eve"]
REPOS = ["web-app", "api-server", "mobile-app", "data-pipeline", "infrastructure", "docs", "frontend", "backend"]
MEMBERS = ["alice", "bob", "charlie", "diana", "eve", "frank", "grace", "henry"]
EVENT_TYPES = ["commit", "pull_request", "deploy"]

def generate_commit_data(owner, repo, member):
    """Generate commit event data"""
    sha = ''.join(random.choices('0123456789abcdef', k=7))
    return {
        "sha": sha,
        "message": random.choice([
            "Fix bug in authentication",
            "Add new feature",
            "Update documentation",
            "Refactor code",
            "Improve performance",
            "Fix typo",
            "Add tests",
            "Update dependencies"
        ]),
        "additions": random.randint(10, 500),
        "deletions": random.randint(5, 300),
        "files_changed": random.randint(1, 20)
    }

def generate_pr_data(owner, repo, member):
    """Generate pull request event data"""
    return {
        "number": random.randint(1, 1000),
        "title": random.choice([
            "Add new feature",
            "Fix critical bug",
            "Update dependencies",
            "Improve documentation",
            "Refactor code",
            "Add tests",
            "Performance improvements"
        ]),
        "state": random.choice(["open", "closed", "merged"]),
        "additions": random.randint(50, 1000),
        "deletions": random.randint(20, 500),
        "files_changed": random.randint(1, 30)
    }

def generate_deploy_data(owner, repo, member):
    """Generate deploy event data"""
    return {
        "id": str(uuid4()),
        "environment": random.choice(["production", "staging", "development"]),
        "status": random.choice(["success", "failure", "pending"]),
        "ref": random.choice(["main", "develop", "release/v1.0"]),
        "sha": ''.join(random.choices('0123456789abcdef', k=7))
    }

def generate_event_data(event_type, owner, owner_type, repo, member):
    """Generate event data based on type"""
    if event_type == "commit":
        return generate_commit_data(owner, repo, member)
    elif event_type == "pull_request":
        return generate_pr_data(owner, repo, member)
    elif event_type == "deploy":
        return generate_deploy_data(owner, repo, member)
    return {}

def generate_event_id(event_type, owner, repo, data):
    """Generate unique event ID"""
    if event_type == "commit":
        return f"{owner}-{repo}-commit-{data.get('sha', '')}"
    elif event_type == "pull_request":
        return f"{owner}-{repo}-pr-{data.get('number', 0)}"
    elif event_type == "deploy":
        return f"{owner}-{repo}-deploy-{data.get('id', '')}"
    return f"{owner}-{repo}-{event_type}-{uuid4()}"

def generate_timestamp(start_date, end_date):
    """Generate random timestamp between start and end dates"""
    time_between = end_date - start_date
    random_seconds = random.randint(0, int(time_between.total_seconds()))
    return start_date + timedelta(seconds=random_seconds)

def main():
    # Generate 1000 events
    num_events = 1000
    
    # Date range: last 90 days
    end_date = datetime.now()
    start_date = end_date - timedelta(days=90)
    
    events = []
    used_ids = set()
    pr_counters = {}  # Track PR numbers per owner-repo
    commit_shas = {}  # Track commit SHAs per owner-repo
    
    for i in range(num_events):
        # Randomly choose owner type
        owner_type = random.choice(["organization", "user"])
        
        if owner_type == "organization":
            owner = random.choice(ORGS)
        else:
            owner = random.choice(USERS)
        
        repo = random.choice(REPOS)
        member = random.choice(MEMBERS)
        event_type = random.choice(EVENT_TYPES)
        
        # Generate unique event data and ID
        max_attempts = 100
        event_id = None
        event_data = None
        
        for attempt in range(max_attempts):
            # Generate event data
            event_data = generate_event_data(event_type, owner, owner_type, repo, member)
            
            # Ensure unique IDs
            if event_type == "pull_request":
                # Use counter for PR numbers to ensure uniqueness
                key = f"{owner}-{repo}"
                if key not in pr_counters:
                    pr_counters[key] = random.randint(1, 1000)
                else:
                    pr_counters[key] += 1
                event_data["number"] = pr_counters[key]
            elif event_type == "commit":
                # Ensure unique SHA per owner-repo
                key = f"{owner}-{repo}"
                if key not in commit_shas:
                    commit_shas[key] = set()
                sha = ''.join(random.choices('0123456789abcdef', k=7))
                attempts_sha = 0
                while sha in commit_shas[key] and attempts_sha < 100:
                    sha = ''.join(random.choices('0123456789abcdef', k=7))
                    attempts_sha += 1
                commit_shas[key].add(sha)
                event_data["sha"] = sha
            
            event_id = generate_event_id(event_type, owner, repo, event_data)
            
            if event_id not in used_ids:
                used_ids.add(event_id)
                break
        
        if event_id is None or event_id in used_ids:
            # Fallback: use UUID to ensure uniqueness
            event_id = f"{owner}-{repo}-{event_type}-{uuid4()}"
            used_ids.add(event_id)
        
        # Generate timestamp
        timestamp = generate_timestamp(start_date, end_date)
        created_at = timestamp + timedelta(seconds=random.randint(0, 3600))
        
        events.append({
            "id": event_id,
            "type": event_type,
            "owner": owner,
            "owner_type": owner_type,
            "repo": repo,
            "member": member,
            "timestamp": timestamp.strftime("%Y-%m-%d %H:%M:%S"),
            "data": json.dumps(event_data),
            "created_at": created_at.strftime("%Y-%m-%d %H:%M:%S")
        })
    
    # Sort by timestamp
    events.sort(key=lambda x: x["timestamp"])
    
    # Write to CSV
    output_file = "test_events.csv"
    with open(output_file, 'w', newline='', encoding='utf-8') as f:
        fieldnames = ["id", "type", "owner", "owner_type", "repo", "member", "timestamp", "data", "created_at"]
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(events)
    
    print(f"Generated {num_events} events in {output_file}")
    print(f"Date range: {start_date.strftime('%Y-%m-%d')} to {end_date.strftime('%Y-%m-%d')}")
    
    # Print statistics
    type_counts = {}
    owner_counts = {}
    for event in events:
        event_type = event["type"]
        type_counts[event_type] = type_counts.get(event_type, 0) + 1
        owner = event["owner"]
        owner_counts[owner] = owner_counts.get(owner, 0) + 1
    
    print("\nEvent type distribution:")
    for event_type, count in sorted(type_counts.items()):
        print(f"  {event_type}: {count}")
    
    print("\nOwner distribution:")
    for owner, count in sorted(owner_counts.items(), key=lambda x: -x[1])[:10]:
        print(f"  {owner}: {count}")

if __name__ == "__main__":
    main()

