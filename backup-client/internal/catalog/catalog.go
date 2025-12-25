package catalog

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// FileEntry represents a backed up file version (supports Time Machine-like versioning)
type FileEntry struct {
	ID          int64     // rowid
	Timestamp   time.Time // tarih - when this version was backed up
	Directory   string    // dizin
	OrigPath    string    // adi (original full path)
	HashedName  string    // yeni_adi (sha224 of path)
	ContentHash string    // hash_degeri (sha256 of content) - detects changes
	Size        int64     // boyu
	PackedSize  int64     // paketli_boyu
}

// FileVersion represents a single version of a file for history display
type FileVersion struct {
	Timestamp   time.Time
	ContentHash string
	Size        int64
}

// Catalog manages the local backup catalog (like butun.katalog in Python)
type Catalog struct {
	db     *sql.DB
	dbPath string
}

// New creates or opens a catalog database
func New(dataDir string) (*Catalog, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "butun.katalog")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	c := &Catalog{db: db, dbPath: dbPath}
	if err := c.initSchema(); err != nil {
		return nil, err
	}

	return c, nil
}

// initSchema creates the table and indexes
func (c *Catalog) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS dosyalar (
			tarih TIMESTAMP,
			dizin TEXT,
			adi TEXT,
			yeni_adi TEXT,
			hash_degeri TEXT,
			boyu INTEGER,
			paketli_boyu INTEGER
		);
		CREATE INDEX IF NOT EXISTS dosyalar_ndx ON dosyalar(tarih, adi);
		CREATE INDEX IF NOT EXISTS hash_ndx ON dosyalar(yeni_adi);
	`
	_, err := c.db.Exec(schema)
	return err
}

// Close closes the database
func (c *Catalog) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// ClearAll deletes all entries from the catalog
func (c *Catalog) ClearAll() error {
	_, err := c.db.Exec(`DELETE FROM dosyalar`)
	return err
}

// AddEntry adds a file entry to the catalog
func (c *Catalog) AddEntry(entry *FileEntry) error {
	_, err := c.db.Exec(
		`INSERT INTO dosyalar (tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Directory, entry.OrigPath, entry.HashedName,
		entry.ContentHash, entry.Size, entry.PackedSize,
	)
	return err
}

// AddEntries adds multiple entries in a transaction
func (c *Catalog) AddEntries(entries []FileEntry) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO dosyalar (tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		_, err := stmt.Exec(e.Timestamp, e.Directory, e.OrigPath, e.HashedName,
			e.ContentHash, e.Size, e.PackedSize)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// UpdatePackedSize updates the packed size for a file
func (c *Catalog) UpdatePackedSize(origPath string, packedSize int64) error {
	_, err := c.db.Exec(
		`UPDATE dosyalar SET paketli_boyu = ? WHERE adi = ?`,
		packedSize, origPath,
	)
	return err
}

// GetLastBackupTime returns the last backup timestamp for a directory
func (c *Catalog) GetLastBackupTime(directory string) (*time.Time, error) {
	var timestamp time.Time
	var err error

	if directory == "" {
		err = c.db.QueryRow(`SELECT MAX(tarih) FROM dosyalar`).Scan(&timestamp)
	} else {
		err = c.db.QueryRow(
			`SELECT MAX(tarih) FROM dosyalar WHERE dizin = ?`, directory,
		).Scan(&timestamp)
	}

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timestamp, nil
}

// FileExists checks if a file path exists in catalog
func (c *Catalog) FileExists(origPath string) (bool, error) {
	var count int
	err := c.db.QueryRow(
		`SELECT COUNT(*) FROM dosyalar WHERE adi = ?`, origPath,
	).Scan(&count)
	return count > 0, err
}

// GetLatestVersion returns the most recent version of a file
func (c *Catalog) GetLatestVersion(origPath string) (*FileEntry, error) {
	var e FileEntry
	err := c.db.QueryRow(
		`SELECT tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu
		 FROM dosyalar WHERE adi = ? ORDER BY tarih DESC LIMIT 1`, origPath,
	).Scan(&e.Timestamp, &e.Directory, &e.OrigPath, &e.HashedName,
		&e.ContentHash, &e.Size, &e.PackedSize)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// NeedsBackup checks if a file needs to be backed up (new or changed)
func (c *Catalog) NeedsBackup(origPath string, currentHash string, currentSize int64) (bool, error) {
	latest, err := c.GetLatestVersion(origPath)
	if err != nil {
		fmt.Printf("[NeedsBackup] error for %s: %v\n", origPath, err)
		return false, err
	}
	// New file
	if latest == nil {
		fmt.Printf("[NeedsBackup] NEW file: %s\n", origPath)
		return true, nil
	}
	// Content changed (hash is different)
	if latest.ContentHash != currentHash {
		fmt.Printf("[NeedsBackup] CHANGED file: %s (old=%s new=%s)\n", origPath, latest.ContentHash[:8], currentHash[:8])
		return true, nil
	}
	// File unchanged
	return false, nil
}

// GetFileHistory returns all versions of a file (Time Machine style)
func (c *Catalog) GetFileHistory(origPath string) ([]FileVersion, error) {
	rows, err := c.db.Query(
		`SELECT tarih, hash_degeri, boyu FROM dosyalar
		 WHERE adi = ? ORDER BY tarih DESC`, origPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []FileVersion
	for rows.Next() {
		var v FileVersion
		if err := rows.Scan(&v.Timestamp, &v.ContentHash, &v.Size); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// GetFilesAtTime returns file states at a specific point in time (for restore)
func (c *Catalog) GetFilesAtTime(targetTime time.Time) ([]FileEntry, error) {
	// Get the latest version of each file that existed at or before targetTime
	rows, err := c.db.Query(
		`SELECT d1.tarih, d1.dizin, d1.adi, d1.yeni_adi, d1.hash_degeri, d1.boyu, d1.paketli_boyu
		 FROM dosyalar d1
		 INNER JOIN (
			 SELECT adi, MAX(tarih) as max_tarih
			 FROM dosyalar
			 WHERE tarih <= ?
			 GROUP BY adi
		 ) d2 ON d1.adi = d2.adi AND d1.tarih = d2.max_tarih`, targetTime,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Timestamp, &e.Directory, &e.OrigPath, &e.HashedName,
			&e.ContentHash, &e.Size, &e.PackedSize); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetFileAtTime returns the version of a specific file at a point in time
// Uses <= comparison with 1 second buffer to handle millisecond precision differences
func (c *Catalog) GetFileAtTime(origPath string, targetTime time.Time) (*FileEntry, error) {
	// Add 1 second buffer to handle millisecond precision (e.g., 01:32:48.000 vs 01:32:48.091)
	searchTime := targetTime.Add(time.Second)
	fmt.Printf("[GetFileAtTime] Looking for: origPath=%s, targetTime=%v, searchTime=%v\n", origPath, targetTime, searchTime)

	var e FileEntry
	err := c.db.QueryRow(
		`SELECT tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu
		 FROM dosyalar
		 WHERE adi = ? AND tarih <= ?
		 ORDER BY tarih DESC
		 LIMIT 1`, origPath, searchTime,
	).Scan(&e.Timestamp, &e.Directory, &e.OrigPath, &e.HashedName,
		&e.ContentHash, &e.Size, &e.PackedSize)
	if err == sql.ErrNoRows {
		fmt.Printf("[GetFileAtTime] No rows found\n")
		return nil, nil
	}
	if err != nil {
		fmt.Printf("[GetFileAtTime] Error: %v\n", err)
		return nil, err
	}
	fmt.Printf("[GetFileAtTime] Found: %+v\n", e)
	return &e, nil
}

// GetFilesInDirAtTime returns all files in a directory with their closest version at or before targetTime
func (c *Catalog) GetFilesInDirAtTime(dirPath string, targetTime time.Time) ([]FileEntry, error) {
	// Add 1 second buffer to handle millisecond precision
	searchTime := targetTime.Add(time.Second)
	fmt.Printf("[GetFilesInDirAtTime] Looking for: dirPath=%s, targetTime=%v\n", dirPath, targetTime)

	// Get all unique file paths under this directory, with their latest version at or before targetTime
	rows, err := c.db.Query(`
		SELECT d1.tarih, d1.dizin, d1.adi, d1.yeni_adi, d1.hash_degeri, d1.boyu, d1.paketli_boyu
		FROM dosyalar d1
		WHERE d1.adi LIKE ? || '%'
		  AND d1.tarih = (
			SELECT MAX(d2.tarih) FROM dosyalar d2
			WHERE d2.adi = d1.adi AND d2.tarih <= ?
		  )
		GROUP BY d1.adi
	`, dirPath, searchTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Timestamp, &e.Directory, &e.OrigPath, &e.HashedName,
			&e.ContentHash, &e.Size, &e.PackedSize); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	fmt.Printf("[GetFilesInDirAtTime] Found %d files\n", len(entries))
	return entries, nil
}

// GetBackupDates returns all unique backup timestamps (for Time Machine UI)
func (c *Catalog) GetBackupDates() ([]time.Time, error) {
	rows, err := c.db.Query(
		`SELECT DISTINCT tarih FROM dosyalar ORDER BY tarih DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		dates = append(dates, t)
	}
	return dates, nil
}

// GetOriginalPath returns the original path for a hashed name
func (c *Catalog) GetOriginalPath(hashedName string) (string, error) {
	hashedName = stripEncExtension(hashedName)
	var origPath string
	err := c.db.QueryRow(
		`SELECT adi FROM dosyalar WHERE yeni_adi = ?`, hashedName,
	).Scan(&origPath)
	return origPath, err
}

// GetAllFiles returns all unique file paths in catalog
func (c *Catalog) GetAllFiles() ([]string, error) {
	rows, err := c.db.Query(`SELECT DISTINCT adi FROM dosyalar`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// CatalogFileInfo represents a file with its version info for Time Machine UI
type CatalogFileInfo struct {
	OrigPath      string    `json:"orig_path"`
	Directory     string    `json:"directory"`
	FileName      string    `json:"file_name"`
	LatestVersion time.Time `json:"latest_version"`
	VersionCount  int       `json:"version_count"`
	Size          int64     `json:"size"`
}

// GetAllFilesWithInfo returns all files with their latest version and version count
func (c *Catalog) GetAllFilesWithInfo() ([]CatalogFileInfo, error) {
	rows, err := c.db.Query(`
		SELECT
			adi,
			dizin,
			MAX(tarih) as latest,
			COUNT(*) as version_count,
			(SELECT boyu FROM dosyalar d2 WHERE d2.adi = d1.adi ORDER BY tarih DESC LIMIT 1) as size
		FROM dosyalar d1
		GROUP BY adi
		ORDER BY dizin, adi
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []CatalogFileInfo
	for rows.Next() {
		var f CatalogFileInfo
		var latestStr string
		if err := rows.Scan(&f.OrigPath, &f.Directory, &latestStr, &f.VersionCount, &f.Size); err != nil {
			return nil, err
		}
		// Parse timestamp string to time.Time
		f.LatestVersion, _ = time.Parse("2006-01-02 15:04:05-07:00", latestStr)
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse("2006-01-02T15:04:05Z", latestStr)
		}
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse(time.RFC3339, latestStr)
		}
		// Extract filename from path
		f.FileName = filepath.Base(f.OrigPath)
		files = append(files, f)
	}
	return files, nil
}

// GetFilesAtTimestamp returns files as they were at a specific timestamp
// For each file, it returns the version that was current at that time (latest version <= timestamp)
func (c *Catalog) GetFilesAtTimestamp(ts time.Time) ([]CatalogFileInfo, error) {
	// Add 1 second buffer to handle millisecond precision differences
	searchTime := ts.Add(time.Second)
	fmt.Printf("[GetFilesAtTimestamp] ts=%v, searchTime=%v\n", ts, searchTime)

	rows, err := c.db.Query(`
		SELECT
			adi,
			dizin,
			tarih,
			boyu,
			(SELECT COUNT(*) FROM dosyalar d2 WHERE d2.adi = d1.adi AND d2.tarih <= ?) as version_count
		FROM dosyalar d1
		WHERE tarih = (
			SELECT MAX(tarih) FROM dosyalar d2
			WHERE d2.adi = d1.adi AND d2.tarih <= ?
		)
		GROUP BY adi
		ORDER BY dizin, adi
	`, searchTime, searchTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []CatalogFileInfo
	for rows.Next() {
		var f CatalogFileInfo
		var latestStr string
		if err := rows.Scan(&f.OrigPath, &f.Directory, &latestStr, &f.Size, &f.VersionCount); err != nil {
			return nil, err
		}
		// Parse timestamp
		f.LatestVersion, _ = time.Parse("2006-01-02 15:04:05-07:00", latestStr)
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse("2006-01-02T15:04:05Z", latestStr)
		}
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse(time.RFC3339, latestStr)
		}
		f.FileName = filepath.Base(f.OrigPath)
		files = append(files, f)
	}
	return files, nil
}

// GetFilesInDirectory returns files in a specific directory for browsing
func (c *Catalog) GetFilesInDirectory(directory string) ([]CatalogFileInfo, error) {
	rows, err := c.db.Query(`
		SELECT
			adi,
			dizin,
			MAX(tarih) as latest,
			COUNT(*) as version_count,
			(SELECT boyu FROM dosyalar d2 WHERE d2.adi = d1.adi ORDER BY tarih DESC LIMIT 1) as size
		FROM dosyalar d1
		WHERE dizin = ?
		GROUP BY adi
		ORDER BY adi
	`, directory)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []CatalogFileInfo
	for rows.Next() {
		var f CatalogFileInfo
		var latestStr string
		if err := rows.Scan(&f.OrigPath, &f.Directory, &latestStr, &f.VersionCount, &f.Size); err != nil {
			return nil, err
		}
		// Parse timestamp string to time.Time
		f.LatestVersion, _ = time.Parse("2006-01-02 15:04:05-07:00", latestStr)
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse("2006-01-02T15:04:05Z", latestStr)
		}
		if f.LatestVersion.IsZero() {
			f.LatestVersion, _ = time.Parse(time.RFC3339, latestStr)
		}
		f.FileName = filepath.Base(f.OrigPath)
		files = append(files, f)
	}
	return files, nil
}

// GetDirectories returns all unique directories
func (c *Catalog) GetDirectories() ([]string, error) {
	rows, err := c.db.Query(`SELECT DISTINCT dizin FROM dosyalar ORDER BY dizin`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, nil
}

// GetStats returns file count, total size, and packed size
func (c *Catalog) GetStats() (count int64, totalSize int64, packedSize int64, err error) {
	err = c.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(boyu), 0), COALESCE(SUM(paketli_boyu), 0) FROM dosyalar`,
	).Scan(&count, &totalSize, &packedSize)
	return
}

// GetFilesForDirectory returns files and their hashed names for restore
func (c *Catalog) GetFilesForDirectory(directory string, beforeTime time.Time) ([]FileEntry, error) {
	rows, err := c.db.Query(
		`SELECT tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu
		 FROM dosyalar
		 WHERE dizin LIKE ? AND tarih <= ?
		 GROUP BY adi
		 ORDER BY tarih DESC`,
		directory+"%", beforeTime,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Timestamp, &e.Directory, &e.OrigPath, &e.HashedName,
			&e.ContentHash, &e.Size, &e.PackedSize); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ExportToFile exports the catalog to a separate database file
func (c *Catalog) ExportToFile(destPath string) error {
	// Create a new database for export
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return err
	}
	defer destDB.Close()

	// Create schema
	schema := `
		CREATE TABLE IF NOT EXISTS dosyalar (
			tarih TIMESTAMP,
			dizin TEXT,
			adi TEXT,
			yeni_adi TEXT,
			hash_degeri TEXT,
			boyu INTEGER,
			paketli_boyu INTEGER
		);
	`
	if _, err := destDB.Exec(schema); err != nil {
		return err
	}

	// Copy all data
	rows, err := c.db.Query(
		`SELECT tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu FROM dosyalar`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := destDB.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO dosyalar (tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var tarih time.Time
		var dizin, adi, yeniAdi, hashDegeri string
		var boyu, paketliBoyu int64

		if err := rows.Scan(&tarih, &dizin, &adi, &yeniAdi, &hashDegeri, &boyu, &paketliBoyu); err != nil {
			tx.Rollback()
			return err
		}

		if _, err := stmt.Exec(tarih, dizin, adi, yeniAdi, hashDegeri, boyu, paketliBoyu); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// ImportFromFile imports entries from another catalog database
func (c *Catalog) ImportFromFile(srcPath string) error {
	srcDB, err := sql.Open("sqlite3", srcPath)
	if err != nil {
		return err
	}
	defer srcDB.Close()

	rows, err := srcDB.Query(
		`SELECT tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu FROM dosyalar`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO dosyalar (tarih, dizin, adi, yeni_adi, hash_degeri, boyu, paketli_boyu)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var tarih time.Time
		var dizin, adi, yeniAdi, hashDegeri string
		var boyu, paketliBoyu int64

		if err := rows.Scan(&tarih, &dizin, &adi, &yeniAdi, &hashDegeri, &boyu, &paketliBoyu); err != nil {
			tx.Rollback()
			return err
		}

		if _, err := stmt.Exec(tarih, dizin, adi, yeniAdi, hashDegeri, boyu, paketliBoyu); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// GetDBPath returns the database file path
func (c *Catalog) GetDBPath() string {
	return c.dbPath
}

func stripEncExtension(name string) string {
	if len(name) > 4 && name[len(name)-4:] == ".enc" {
		return name[:len(name)-4]
	}
	return name
}

// SessionCatalog creates a new session-specific catalog (like YYYYMMDD-HHMMSS.katalog)
func NewSessionCatalog(dataDir, sessionID string) (*Catalog, error) {
	dbPath := filepath.Join(dataDir, fmt.Sprintf("%s.katalog", sessionID))
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	c := &Catalog{db: db, dbPath: dbPath}
	if err := c.initSchema(); err != nil {
		return nil, err
	}

	return c, nil
}
