package service

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
	"log"
	"time"

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

	// GetLatestRecord retrieves the latest version of a record by its ID for v2 api
	GetLatestRecord(ctx context.Context, id int) (entity.V2Record, error)

	// GetRecordByVersion retrieves a specific version of a record by its ID for v2 api
	GetRecordByVersion(ctx context.Context, id, version int) (entity.V2Record, error)

	// ListAllVersions retrieves all versions of a record by its ID for v2 api
	ListAllVersions(ctx context.Context, id int) ([]int, error)

	// CreateUpdateRecordVersioned creates/updates a record with versioning support.
	CreateUpdateRecordVersioned(ctx context.Context, id int, data map[string]*string) (entity.V2Record, error)
}

// SQLite implementation of RecordService.
type SQLiteRecordService struct {
    db *sql.DB
}

//New instance of SQLiteVersionedRecordService with a database connection.
//It will create the records table if it does not exist.
func NewSQLiteRecordService(dataSource string) (*SQLiteRecordService, error) {
    db, err := sql.Open("sqlite3", dataSource)
    if err != nil {
        return nil, err
    }
	
	// Create the records table if it does not exist
    query := `
	CREATE TABLE IF NOT EXISTS records (
    	id     INTEGER NOT NULL,
    	key    TEXT NOT NULL,
    	value  TEXT NOT NULL
	);`
    _, err = db.Exec(query)
    if err != nil {
        return nil, err
    }

	// Alter the records table to add versioning and timestamps
    alterStatements := []string{
        `ALTER TABLE records ADD COLUMN version INTEGER NOT NULL DEFAULT 1;`,
        `ALTER TABLE records ADD COLUMN effective_at DATETIME;`, // This column will be used to track when the record was effective
        `ALTER TABLE records ADD COLUMN created_at DATETIME;`, // This column will be used to track when the record was created
        `CREATE INDEX IF NOT EXISTS idx_record_id_version ON records (id, version);`,
    }
    for _, stmt := range alterStatements {
        _, _ = db.Exec(stmt) // Ignore error if column already exists
    }
    return &SQLiteRecordService{db: db}, nil
}

// GetRecord retrieves a record by its ID for v1 api
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

// CreateRecord inserts a new record into the database for v1 api
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

// UpdateRecord updates an existing record with new key-value pairs for v1 api
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

// GetLatestRecord retrieves the latest version of a record by its ID for v2 api
func (s *SQLiteRecordService) GetLatestRecord(ctx context.Context, id int) (entity.V2Record, error) {
	// Get the latest version and its effective_at timestamp
    var version int

    err := s.db.QueryRowContext(ctx, `
        SELECT MAX(version) FROM records WHERE id = ?
    `, id).Scan(&version)
    if err != nil {
		log.Printf("Error retrieving latest version for record ID %d: %v", id, err)
        return entity.V2Record{}, fmt.Errorf("could not determine latest version: %w", err)
    }

	return s.GetRecordByVersion(ctx, id, version)
}

// GetRecordByVersion retrieves a specific version of a record by its ID for v2 api
func (s *SQLiteRecordService) GetRecordByVersion(ctx context.Context, id, version int) (entity.V2Record, error) {
	if version <= 0 {
		return entity.V2Record{}, fmt.Errorf("invalid version number: %d", version)
	}
	// Get metadata (effective_at, created_at) from one representative row
    var effectiveAt sql.NullTime
	var createdAt sql.NullTime
    err := s.db.QueryRowContext(ctx, `
        SELECT effective_at, created_at
        FROM records
        WHERE id = ? AND version = ?
        LIMIT 1
    `, id, version).Scan(&effectiveAt, &createdAt)

    if err != nil {
		log.Printf("Error fetching metadata for record ID %d, version %d: %v", id, version, err)
        return entity.V2Record{}, fmt.Errorf("failed to fetch metadata: %w", err)
    }

    query := `SELECT key, value FROM records WHERE id = ? AND version = ?`
    rows, err := s.db.QueryContext(ctx, query, id, version)
    if err != nil {
	    log.Printf("Error retrieving record data for ID %d, version %d: %v", id, version, err)
        return entity.V2Record{}, err
    }
    defer rows.Close()

    data := map[string]string{}
    for rows.Next() {
        var k, v string
        if err := rows.Scan(&k, &v); err != nil {
            return entity.V2Record{}, err
        }
        data[k] = v
    }

    if len(data) == 0 {
        return entity.V2Record{}, fmt.Errorf("no record found for version %d", version)
    }
    return entity.V2Record{ID: id, Version: version, Effective_at: effectiveAt.Time, Created_at: createdAt.Time, Data: data}, nil
}

// ListAllVersions retrieves all versions of a record by its ID for v2 api
func (s *SQLiteRecordService) ListAllVersions(ctx context.Context, id int) ([]int, error) {
    query := `SELECT DISTINCT version FROM records WHERE id = ? ORDER BY version ASC`
    rows, err := s.db.QueryContext(ctx, query, id)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var versions []int
    for rows.Next() {
        var v int
        if err := rows.Scan(&v); err != nil {
            return nil, err
        }
        versions = append(versions, v)
    }
    return versions, nil
}

// CreateUpdateRecordVersioned creates/updates a record with versioning support.
func (s *SQLiteRecordService) CreateUpdateRecordVersioned(ctx context.Context, id int, data map[string]*string) (entity.V2Record, error)  {
    version, err := s.nextVersion(ctx, id)
    if err != nil {
        return entity.V2Record{}, err
    }

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return entity.V2Record{}, err
    }

    var effectiveAt time.Time
    if val, ok := data["effective_at"]; ok && val != nil {
        parsedTime, err := time.Parse(time.RFC3339, *val)
        if err != nil {
            log.Printf("Invalid effective_at: %v", err)
            return entity.V2Record{}, err
        } else {
            log.Printf("Parsed timestamp: %v", parsedTime)
            effectiveAt = parsedTime
        }
    } else {
        log.Println("No effective_at provided, defaulting to time.Now()")
        effectiveAt = time.Now()
    }

	for k, v := range data {
		if v == nil || k == "effective_at" {
			// If value is nil, we skip inserting it
			continue
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO records (id, key, value, version, effective_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, k, v, version, effectiveAt, time.Now())
		if err != nil {
			tx.Rollback()
			return entity.V2Record{}, err
		}
	}
    err = tx.Commit()

    if err != nil {
        return entity.V2Record{}, err
    }
    return s.GetRecordByVersion(ctx, id, version) // Return latest after create/update
}

// nextVersion retrieves the next version number for a record by its ID.
func (s *SQLiteRecordService) nextVersion(ctx context.Context, id int) (int, error) {
    var v int
    err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM records WHERE id = ?", id).Scan(&v)
    return v + 1, err
}