#!/usr/bin/env python3
"""
scripts/search_emails.py - Search cached emails in SQLite database.
Can be executed by Antigravity CLI directly in the workspace.
"""
import sys
import sqlite3
import os

def main():
    query = " ".join(sys.argv[1:]).strip()
    if not query:
        print("Usage: python3 scripts/search_emails.py <search_query>")
        sys.exit(1)

    candidates = [
        os.environ.get("DB_PATH", ""),
        "/opt/email-agent/data/emails.db",
        "../data/emails.db",
        "data/emails.db"
    ]
    db_path = next((p for p in candidates if p and os.path.exists(p)), None)
    if not db_path:
        print(f"Error: emails.db not found in candidates: {candidates}")
        sys.exit(1)

    conn = sqlite3.connect(db_path)
    cur = conn.cursor()

    results = []
    # 1. Try FTS5
    try:
        sanitized = query.replace('"', '""')
        cur.execute("""
            SELECT e.date, e.sender, e.subject, e.body_text
            FROM emails e
            JOIN emails_fts fts ON e.message_id = fts.message_id
            WHERE emails_fts MATCH ?
            ORDER BY e.date DESC
            LIMIT 10
        """, (f'"{sanitized}"',))
        results = cur.fetchall()
    except Exception:
        pass

    # 2. Fallback to LIKE
    if not results:
        like_pat = f"%{query}%"
        cur.execute("""
            SELECT date, sender, subject, body_text
            FROM emails
            WHERE subject LIKE ? OR body_text LIKE ?
            ORDER BY date DESC
            LIMIT 10
        """, (like_pat, like_pat))
        results = cur.fetchall()

    if not results:
        print(f"No emails found matching query: {query}")
        return

    print(f"Found {len(results)} matching email(s) across owner accounts:\n")
    for r in results:
        date_str = str(r[0])
        sender = r[1]
        subject = r[2]
        body = (r[3][:300] + "...") if len(r[3]) > 300 else r[3]
        print(f"[{date_str}] From: {sender} | Subject: {subject}")
        print(f"Content: {body}\n{'-'*50}")

if __name__ == "__main__":
    main()
