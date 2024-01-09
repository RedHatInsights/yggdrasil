package messagejournal

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"reflect"
	"text/template"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/ipc"
)

//go:embed migrations/*.sql
var embeddedMigrationData embed.FS

// MessageJournal is a data structure representing the collection
// of message journal entries received from worker emitted events and messages.
// It also stores the date time of when the journal was initialized to track
// events and messages in the active session.
type MessageJournal struct {
	database      *sql.DB
	initializedAt time.Time
	lastUpdated   time.Time
}

// Filter is a data structure representing the filtering options
// that are used when message journal entries are retrieved by yggctl.
type Filter struct {
	Persistent     bool
	MessageID      string
	Worker         string
	Since          string
	Until          string
	TruncateFields map[string]int
}

type errorJournal struct {
	err error
}

func (e *errorJournal) Error() string {
	return fmt.Sprintf("%v", e.err)
}

func (e *errorJournal) Is(o error) bool {
	return reflect.TypeOf(e) == reflect.TypeOf(o)
}

// Open initializes a message journal sqlite database consisting
// of a persistent table that maintains journal entries across sessions.
func Open(databaseFilePath string) (*MessageJournal, error) {
	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return nil, fmt.Errorf("database object not created: %w", err)
	}
	if err = migrateMessageJournalDB(db, databaseFilePath); err != nil {
		return nil, fmt.Errorf("database migration error: %w", err)
	}

	initTime := time.Now().UTC()
	messageJournal := MessageJournal{database: db, initializedAt: initTime, lastUpdated: initTime}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("message journal database not connected: %w", err)
	}

	return &messageJournal, nil
}

// migrateMessageJournalDB handles the migration of the message journal
// database and ensures the schema is up to date on each session start.
func migrateMessageJournalDB(db *sql.DB, databaseFilePath string) error {
	databaseDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("database driver not initialized: %w", err)
	}
	migrationDriver, err := iofs.New(embeddedMigrationData, "migrations")
	if err != nil {
		return fmt.Errorf("embedded migration data not found: %w", err)
	}
	migration, err := migrate.NewWithInstance(
		"iofs",
		migrationDriver,
		databaseFilePath,
		databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("database migration not initialized: %w", err)
	}
	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("database migration failed: %w", err)
	}
	return nil
}

// AddEntry adds a new message journal entry to the persistent table
// in the database.
func (j *MessageJournal) AddEntry(entry yggdrasil.WorkerMessage) error {
	const insertEntryTemplate string = `INSERT INTO journal (
		message_id, sent, worker_name, response_to, worker_event, worker_data)
		values (?,?,?,?,?,?)`

	insertAction, err := j.database.Prepare(insertEntryTemplate)
	if err != nil {
		return fmt.Errorf(
			"cannot prepare statement for 'journal' table: %w",
			err,
		)
	}

	// JSON-encode the event data to make it compatible for database insertion.
	encodedEventData, err := json.Marshal(entry.WorkerEvent.EventData)
	if err != nil {
		return fmt.Errorf(
			"cannot prepare statement for 'journal' table: %w",
			err,
		)
	}

	persistentResult, err := insertAction.Exec(
		entry.MessageID,
		entry.Sent,
		entry.WorkerName,
		entry.ResponseTo,
		entry.WorkerEvent.EventName,
		string(encodedEventData),
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert journal entry into 'journal' table: %w",
			err,
		)
	}

	entryID, err := persistentResult.LastInsertId()
	if err != nil {
		return fmt.Errorf(
			"could not select last insert ID '%v' for 'journal' table: %w",
			entryID,
			err,
		)
	}
	j.lastUpdated = time.Now().UTC()

	log.Debugf("new message journal entry (id: %v) added: '%v'", entryID, entry.MessageID)

	return nil
}

// GetEntries retrieves a list of all the journal entries in the message journal database
// that meet the criteria of the provided message journal filter.
func (j *MessageJournal) GetEntries(filter Filter) ([]map[string]string, error) {
	entries := []map[string]string{}
	queryString, err := buildDynamicGetEntriesQuery(filter, j.initializedAt)
	if err != nil {
		return nil, fmt.Errorf("cannot build dynamic sql query: %w", err)
	}

	preparedQuery, err := j.database.Prepare(queryString)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare query to retrieve journal entries: %w", err)
	}

	rows, err := preparedQuery.Query()
	if err != nil {
		return nil, fmt.Errorf("cannot execute query to retrieve journal entries: %w", err)
	}

	for rows.Next() {
		var rowID int
		var messageID string
		var sent time.Time
		var workerName string
		var responseTo string
		var workerEvent uint
		var workerEventData string

		err := rows.Scan(
			&rowID,
			&messageID,
			&sent,
			&workerName,
			&responseTo,
			&workerEvent,
			&workerEventData,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot scan journal entry columns: %w", err)
		}

		// Truncate data fields
		if len(filter.TruncateFields) > 0 {
			err := truncateEventDataFields(&workerEventData, filter.TruncateFields)
			if err != nil {
				return nil, fmt.Errorf("cannot truncate data field: %w", err)
			}
		}

		// Convert the entry properties into a string format and append to the list of entries.
		newMessage := map[string]string{
			"message_id":   messageID,
			"sent":         sent.String(),
			"worker_name":  workerName,
			"response_to":  responseTo,
			"worker_event": ipc.WorkerEventName(workerEvent).String(),
			"worker_data":  workerEventData,
		}
		entries = append(entries, newMessage)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("cannot iterate queried journal entries: %w", err)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot close journal entry rows: %w", err)
	}

	if len(entries) == 0 {
		return nil, &errorJournal{fmt.Errorf("no journal entries found")}
	}

	return entries, nil
}

// truncateEventDataFields is a utility method that truncates message journal event
// data fields by the lengths specified in the journal filter.
// This process requires unmarshalling the worker event data,
// extracting the specified field (if any), and truncating the
// content of the field to the maximum length.
func truncateEventDataFields(workerEventData *string, truncateOpts map[string]int) error {
	var eventData map[string]string
	err := json.Unmarshal([]byte(*workerEventData), &eventData)
	if err != nil {
		return fmt.Errorf("cannot unmarshal worker event data: %w", err)
	}

	for field, length := range truncateOpts {
		fieldContent, ok := eventData[field]
		if !ok {
			log.Debugf("cannot find specified field to truncate: %v", field)
			continue
		}
		if len(fieldContent) >= length && length >= 0 {
			eventData[field] = fmt.Sprintf("%+v...", eventData[field][:length])
		}
	}

	truncatedEventData, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf(
			"cannot marshal worker event data after truncating data: %w",
			err,
		)
	}
	*workerEventData = string(truncatedEventData)
	return nil
}

// buildDynamicGetEntriesQuery is a utility method that builds the dynamic sql query
// required to filter journal entry messages from the message journal database
// when they are retrieved in the 'GetEntries' method.
func buildDynamicGetEntriesQuery(filter Filter, initializedAt time.Time) (string, error) {
	queryTemplate := template.New("dynamicGetEntriesQuery")
	queryTemplateParse, err := queryTemplate.Parse(
		`SELECT * FROM journal ` +
			`{{if .MessageID}}INTERSECT SELECT * FROM journal WHERE message_id='{{.MessageID}}' {{end}}` +
			`{{if .Worker}}INTERSECT SELECT * FROM journal WHERE worker_name='{{.Worker}}' {{end}}` +
			`{{if .Since}}INTERSECT SELECT * FROM journal WHERE sent>='{{.Since}}' {{end}}` +
			`{{if .Until}}INTERSECT SELECT * FROM journal WHERE sent<='{{.Until}}' {{end}}` +
			`{{if not .Persistent}}INTERSECT SELECT * FROM journal WHERE sent>='{{.InitializedAt}}' {{end}}` +
			`ORDER BY sent`,
	)
	if err != nil {
		return "", fmt.Errorf("cannot parse query template parameters: %w", err)
	}

	var compiledQuery bytes.Buffer
	err = queryTemplateParse.Execute(&compiledQuery,
		struct {
			InitializedAt string
			Persistent    bool
			MessageID     string
			Worker        string
			Since         string
			Until         string
		}{
			initializedAt.String(), filter.Persistent,
			filter.MessageID, filter.Worker, filter.Since, filter.Until,
		})
	if err != nil {
		return "", fmt.Errorf("cannot compile query template: %w", err)
	}
	return compiledQuery.String(), nil
}
