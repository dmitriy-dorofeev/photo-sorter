// Package state управляет межзапусковым состоянием в BoltDB.
// Хранит метаданные об уже обработанных файлах для инкрементальных запусков.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
	"photo-sorter/internal/scanner"
)

const (
	stateDirName      = ".photo-sorter"
	stateFileName     = "state.bolt"
	bucketName        = "files"
	faceAliasesBucket = "face_aliases"
)

// Record содержит метаданные обработанного файла.
type Record struct {
	SourcePath  string    // абсолютный путь к исходному файлу
	Size        int64     // размер файла
	ModTime     time.Time // время модификации
	FastHash    uint64    // быстрый хеш (первые 64KB + последние 64KB)
	FullHash    uint64    // полный xxhash (если был вычислен deduper'ом)
	TargetPath  string    // куда скопировали
	ProcessedAt time.Time // время обработки
}

// State предоставляет доступ к хранилищу состояния.
type State struct {
	db *bbolt.DB
}

// Open открывает (или создаёт) state-файл в целевой папке.
func Open(targetDir string) (*State, error) {
	dir := filepath.Join(targetDir, stateDirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	path := filepath.Join(dir, stateFileName)
	db, err := bbolt.Open(path, 0644, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open state db: %w", err)
	}
	// Создаём bucket, если его нет.
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists([]byte(faceAliasesBucket))
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init state bucket: %w", err)
	}
	return &State{db: db}, nil
}

// Close закрывает базу данных.
func (s *State) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Filter разделяет отсканированные файлы на те, что изменились (или новые),
// и те, что остались без изменений с прошлого запуска.
// Неизменившиеся файлы возвращаются как записи из state — из них можно извлечь
// известные хеши для межзапусковой дедупликации.
func (s *State) Filter(files []scanner.FileInfo) (toProcess []scanner.FileInfo, unchanged []Record, err error) {
	if s.db == nil {
		return files, nil, nil
	}
	err = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			toProcess = append(toProcess, files...)
			return nil
		}
		for _, f := range files {
			key := []byte(f.Path)
			val := b.Get(key)
			if val == nil {
				toProcess = append(toProcess, f)
				continue
			}
			var rec Record
			if err := json.Unmarshal(val, &rec); err != nil {
				// Повреждённая запись — считаем файл новым.
				toProcess = append(toProcess, f)
				continue
			}
			if rec.Size == f.Size && rec.ModTime.Equal(f.ModTime) {
				unchanged = append(unchanged, rec)
			} else {
				toProcess = append(toProcess, f)
			}
		}
		return nil
	})
	return toProcess, unchanged, err
}

// RecordsBySize возвращает все записи из state с заданным размером.
func (s *State) RecordsBySize(size int64) ([]Record, error) {
	var res []Record
	if s.db == nil {
		return res, nil
	}
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var rec Record
			if err := json.Unmarshal(v, &rec); err != nil {
				return nil // пропускаем повреждённые записи
			}
			if rec.Size == size {
				res = append(res, rec)
			}
			return nil
		})
	})
	return res, err
}

// Update записывает или обновляет записи атомарной транзакцией.
func (s *State) Update(records []Record) error {
	if s.db == nil || len(records) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		for _, rec := range records {
			rec.ProcessedAt = time.Now()
			val, err := json.Marshal(rec)
			if err != nil {
				return fmt.Errorf("marshal record for %s: %w", rec.SourcePath, err)
			}
			if err := b.Put([]byte(rec.SourcePath), val); err != nil {
				return fmt.Errorf("put record for %s: %w", rec.SourcePath, err)
			}
		}
		return nil
	})
}

// Cleanup удаляет записи для source-файлов, которых больше нет.
func (s *State) Cleanup(existingPaths []string) error {
	if s.db == nil {
		return nil
	}
	set := make(map[string]struct{}, len(existingPaths))
	for _, p := range existingPaths {
		set[p] = struct{}{}
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		var toDelete [][]byte
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if _, ok := set[string(k)]; !ok {
				toDelete = append(toDelete, k)
			}
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetFaceAliases возвращает все сохранённые face-alias'ы.
func (s *State) GetFaceAliases() (map[string]string, error) {
	result := make(map[string]string)
	if s.db == nil {
		return result, nil
	}
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(faceAliasesBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			result[string(k)] = string(v)
			return nil
		})
	})
	return result, err
}

// UpdateFaceAliases сохраняет face-alias'ы атомарно (дополняет существующие).
func (s *State) UpdateFaceAliases(aliases map[string]string) error {
	if s.db == nil || len(aliases) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(faceAliasesBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", faceAliasesBucket)
		}
		for k, v := range aliases {
			if err := b.Put([]byte(k), []byte(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Reset удаляет state-файл из целевой папки.
func Reset(targetDir string) error {
	path := filepath.Join(targetDir, stateDirName, stateFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state file: %w", err)
	}
	// Пробуем удалить пустую директорию .photo-sorter.
	dir := filepath.Join(targetDir, stateDirName)
	_ = os.Remove(dir)
	return nil
}
