// +build !windows
package state

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// We use a symlink lock technique for unix systems so that we aren't dependent
// on filesystem settings or compatibility.
func (s *LocalState) lock(reason string) error {
	stateDir, stateName := filepath.Split(s.Path)
	if stateName == "" {
		panic("empty state file path")
	}

	if stateName[0] == '.' {
		stateName = stateName[1:]
	}

	s.lockPath = filepath.Join(stateDir, fmt.Sprintf(".%s.lock", stateName))
	s.lockInfoPath = filepath.Join(stateDir, fmt.Sprintf(".%s.lock.info", stateName))

	info := lockInfo{}

	// Create the broken symlink first, because that's the atomic operation.
	// Once we have a symlink, then we can fill in the info at our leisure.
	err := os.Symlink(s.lockInfoPath, s.lockPath)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to lock state file %q: %s", s.Path, err)
		}

		infoData, err := ioutil.ReadFile(s.lockInfoPath)
		if err != nil {
			// TODO: try again just to make sure we're not racing another
			//       process and see if we can't get info
			return fmt.Errorf("state file %q locked, but no info found", s.Path)
		}

		err = json.Unmarshal(infoData, &info)
		if err != nil {
			return fmt.Errorf("could not unmarshal lock info: %s", err)
		}

		// TODO: Should we automatically unlock after expiration?
		//       This would align with lock implementations where the lock
		//       disappears after expiration.
		return fmt.Errorf("state file %q locked, created:%s, expires:%s, reason:%s",
			s.Path, info.Time, info.Expires, info.Reason)
	}

	info.Time = time.Now()
	info.Expires = info.Time.Add(time.Hour)
	info.Reason = reason
	infoData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("could not marshal lock info for %q: %s", s.Path, err)
	}

	err = ioutil.WriteFile(s.lockInfoPath, infoData, 0600)
	if err != nil {
		fmt.Errorf("could not write lock info for %q: %s", s.Path, err)
	}

	return nil
}

func (s *LocalState) unlock() error {
	if s.lockPath != "" {
		os.Remove(s.lockPath)
	}

	if s.lockInfoPath != "" {
		os.Remove(s.lockInfoPath)
	}
	return nil
}
