package service

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
	"log"

    "github.com/rainbowmga/timetravel/entity"
    _ "github.com/mattn/go-sqlite3"
)

var ErrRecordDoesNotExist = errors.New("record with that id does not exist")
var ErrRecordIDInvalid = errors.New("record id must >= 0")
var ErrRecordAlreadyExists = errors.New("record already exists")

// Implements method to get, create, and update record data.
type RecordService interface {

	// GetRecord will retrieve an record.
	GetRecord(ctx context.Context, id int) (entity.Record, error)

	// CreateRecord will insert a new record.
	//
	// If it a record with that id already exists it will fail.
	CreateRecord(ctx context.Context, record entity.Record) error

	// UpdateRecord will change the internal `Map` values of the record if they exist.
	// if the update[key] is null it will delete that key from the record's Map.
	//
	// UpdateRecord will error if id <= 0 or the record does not exist with that id.
	UpdateRecord(ctx context.Context, id int, updates map[string]*string) (entity.Record, error)
}

// SQLite implementation of RecordService.
type SQLiteRecordService struct {
    db *sql.DB
}

//New instance of SQLiteVersionedRecordService with a database connection.
//It will create the records table if it does not exist.
func NewSQLiteVersionedRecordService(dataSource string) (*SQLiteRecordService, error) {
    db, err := sql.Open("sqlite3", dataSource)
    if err != nil {
        return nil, err
    }
    query := `
    DROP TABLE IF EXISTS records;
    `
    _, err = db.Exec(query)
    if err != nil {
        return nil, err
    }
    query = `
	CREATE TABLE IF NOT EXISTS records (
    	id     INTEGER NOT NULL,
    	key    TEXT NOT NULL,
    	value  TEXT,
    	PRIMARY KEY (id, key)
	);`
    _, err = db.Exec(query)
    if err != nil {
        return nil, err
    }
    return &SQLiteRecordService{db: db}, nil
}

func (s *SQLiteRecordService) GetRecord(ctx context.Context, id int) (entity.Record, error) {
	log.Printf("Retrieving record with ID: %d", id)
    query := `SELECT key, value FROM records WHERE id = ?`
    rows, err := s.db.QueryContext(ctx, query, id)
    if err != nil {
        return entity.Record{}, err
    }
    defer rows.Close()

    data := make(map[string]string)
    for rows.Next() {
        var k, v string
        if err := rows.Scan(&k, &v); err != nil {
            return entity.Record{}, err
        }
        data[k] = v
    }

    if len(data) == 0 {
        return entity.Record{}, ErrRecordDoesNotExist
    }
    return entity.Record{ID: id, Data: data}, nil
}

func (s *SQLiteRecordService) CreateRecord(ctx context.Context, rec entity.Record) error {
	log.Printf("Creating record with ID: %d", rec.ID)
	id := rec.ID
	if id <= 0 {
		return ErrRecordIDInvalid
	}
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

	log.Printf("Inserting record with ID: %d", rec.ID)
    // Insert all keys
    for k, v := range rec.Data {
        if _, err := tx.ExecContext(ctx,
            `INSERT INTO records (id, key, value) VALUES (?, ?, ?)`,
            rec.ID, k, v); err != nil {
            return err
        }
    }

    return tx.Commit()
}

func (s *SQLiteRecordService) UpdateRecord(ctx context.Context, id int, updates map[string]*string) (entity.Record, error) {
    log.Printf("Updating record with ID: %d", id)
	if id <= 0 {
		return entity.Record{}, ErrRecordIDInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return entity.Record{}, err
    }
    defer tx.Rollback()

	// Throw error if id does not exist
    var exists bool
    err = tx.QueryRowContext(ctx,
        `SELECT EXISTS (SELECT 1 FROM records WHERE id = ?)`,
        id).Scan(&exists)
    if err != nil {
        return entity.Record{}, err
    }
	if !exists {
		return entity.Record{}, ErrRecordDoesNotExist
	}

    for k, v := range updates {
        if v == nil {
			// delete only if key exists
            res, err := tx.ExecContext(ctx,
                `DELETE FROM records WHERE id = ? AND key = ?`, id, k)
            if err != nil {
                return entity.Record{}, err
            }
            if rows, _ := res.RowsAffected(); rows == 0 {
                return entity.Record{}, fmt.Errorf("key %q does not exist", k)
            }
        } else {
			// Insert new or update existing key-value pair
            if _, err := tx.ExecContext(ctx,
                `INSERT INTO records (id, key, value) VALUES (?, ?, ?)
                 ON CONFLICT(id, key) DO UPDATE SET value = excluded.value`,
                id, k, *v); err != nil {
                return entity.Record{}, fmt.Errorf("failed to upsert key %q: %w", k, err)
            }
			if err != nil {
                return entity.Record{}, err
			}	
        }
    }

    if err := tx.Commit(); err != nil {
        return entity.Record{}, err
    }

    // Retrieve the updated record to return
    updatedRecord, err := s.GetRecord(ctx, id)
    if err != nil {
        return entity.Record{}, err
    }
    return updatedRecord, nil
}