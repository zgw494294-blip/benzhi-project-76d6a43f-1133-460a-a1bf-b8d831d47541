package filecase

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Store) ValidateAudit(id string) error {
	f, e := os.Open(filepath.Join(s.dir, id+".audit.jsonl"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	last := int64(0)
	for sc.Scan() {
		var v struct {
			Version int64 `json:"version"`
		}
		if json.Unmarshal(sc.Bytes(), &v) != nil || v.Version <= last {
			return ErrCorruptAudit
		}
		last = v.Version
	}
	return sc.Err()
}
