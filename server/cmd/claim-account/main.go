package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	"lunar-tear/server/internal/database"
)

type userRef struct {
	table  string
	column string
}

func userReferences(tx *sql.Tx) ([]userRef, error) {
	rows, err := tx.Query(`SELECT m.name, f."from" FROM sqlite_master m, pragma_foreign_key_list(m.name) f
		WHERE m.type = 'table' AND f."table" = 'users'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []userRef
	for rows.Next() {
		var r userRef
		if err := rows.Scan(&r.table, &r.column); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func main() {
	dbPath := flag.String("db", "db/game.db", "SQLite database path")
	name := flag.String("name", "", "In-game player name to look up in user_profile (required)")
	flag.Parse()

	if *name == "" {
		log.Fatal("--name flag is required")
	}

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	var targetId int64
	err = db.QueryRow(`SELECT user_id FROM user_profile WHERE name = ?`, *name).Scan(&targetId)
	if err == sql.ErrNoRows {
		log.Fatalf("no user found with name %q", *name)
	}
	if err != nil {
		log.Fatalf("query user_profile: %v", err)
	}

	var targetUuid string
	err = db.QueryRow(`SELECT uuid FROM users WHERE user_id = ?`, targetId).Scan(&targetUuid)
	if err != nil {
		log.Fatalf("query target uuid: %v", err)
	}

	var latestId int64
	var latestUuid string
	err = db.QueryRow(`SELECT user_id, uuid FROM users ORDER BY user_id DESC LIMIT 1`).Scan(&latestId, &latestUuid)
	if err != nil {
		log.Fatalf("query latest user: %v", err)
	}

	if targetId == latestId {
		log.Printf("user %q (id=%d) is already the most recent user, nothing to do", *name, targetId)
		return
	}

	log.Printf("target:  id=%d uuid=%s (name=%q)", targetId, targetUuid, *name)
	log.Printf("latest:  id=%d uuid=%s (will be deleted)", latestId, latestUuid)

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback()

	refs, err := userReferences(tx)
	if err != nil {
		log.Fatalf("list tables referencing users: %v", err)
	}
	for _, r := range refs {
		if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE "%s" = ?`, r.table, r.column), latestId); err != nil {
			log.Fatalf("delete from %s: %v", r.table, err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM users WHERE user_id = ?`, latestId); err != nil {
		log.Fatalf("delete latest user: %v", err)
	}

	if _, err := tx.Exec(`UPDATE users SET uuid = ? WHERE user_id = ?`, latestUuid, targetId); err != nil {
		log.Fatalf("update target uuid: %v", err)
	}

	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, targetId); err != nil {
		log.Fatalf("delete stale sessions: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("claimed account:\n")
	fmt.Printf("  user %d (%s): uuid changed %s -> %s\n", targetId, *name, targetUuid, latestUuid)
	fmt.Printf("  user %d: deleted\n", latestId)
}
