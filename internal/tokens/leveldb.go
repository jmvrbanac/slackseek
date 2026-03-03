package tokens

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// WorkspaceToken holds the raw fields extracted from the Slack LevelDB store
// before the cookie is merged into a full Workspace.
type WorkspaceToken struct {
	Name  string
	Token string
	URL   string
}

// localConfigV2Payload is the JSON structure stored under the
// `*localConfig_v2` key in Slack's LevelDB Local Storage.
type localConfigV2Payload struct {
	Teams map[string]struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"teams"`
}

// copyDir copies all regular files from src into dst (which must already exist).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", src, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	return nil
}

// snapshotLevelDB copies leveldbDir to a temp directory, opens the snapshot
// with RecoverFile (tolerates races with the live Slack process), and returns
// the temp dir path and an open DB handle. On error the temp dir is cleaned up.
// The caller is responsible for calling os.RemoveAll(tmpDir) and db.Close().
func snapshotLevelDB(leveldbDir string) (tmpDir string, db *leveldb.DB, err error) {
	tmpDir, err = os.MkdirTemp("", "slackseek-leveldb-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	if copyErr := copyDir(leveldbDir, tmpDir); copyErr != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("snapshot LevelDB: %w", copyErr)
	}
	db, err = leveldb.RecoverFile(tmpDir, nil)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf(
			"open Slack LevelDB at %s: %w\n"+
				"Ensure the Slack desktop application is installed.",
			leveldbDir, err,
		)
	}
	return tmpDir, db, nil
}

// findLocalConfigPayload scans db for the `localConfig_v2` key and returns
// the parsed payload, or an error when the key is absent or unreadable.
func findLocalConfigPayload(db *leveldb.DB, leveldbDir string) (*localConfigV2Payload, error) {
	var payload *localConfigV2Payload
	iter := db.NewIterator(&util.Range{}, nil)
	for iter.Next() {
		if !strings.Contains(string(iter.Key()), "localConfig_v2") {
			continue
		}
		raw, err := decodeLocalStorageValue(iter.Value())
		if err != nil {
			continue
		}
		var p localConfigV2Payload
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		payload = &p
		break
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterating LevelDB: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf(
			"localConfig_v2 key not found in LevelDB at %s: "+
				"Slack may not be installed or the storage path is wrong",
			leveldbDir,
		)
	}
	return payload, nil
}

// buildWorkspaceList converts a parsed localConfig_v2 payload to WorkspaceTokens.
func buildWorkspaceList(payload *localConfigV2Payload, leveldbDir string) ([]WorkspaceToken, error) {
	workspaces := make([]WorkspaceToken, 0, len(payload.Teams))
	for _, team := range payload.Teams {
		if team.Token == "" || team.Name == "" {
			continue
		}
		workspaces = append(workspaces, WorkspaceToken{Name: team.Name, Token: team.Token, URL: team.URL})
	}
	if len(workspaces) == 0 {
		return nil, fmt.Errorf(
			"no workspace tokens found in Slack LevelDB at %s: "+
				"ensure you are logged in to at least one Slack workspace",
			leveldbDir,
		)
	}
	return workspaces, nil
}

// ExtractWorkspaceTokens copies the Slack LevelDB at leveldbDir to a temp
// directory and reads the `localConfig_v2` entry from the snapshot.  Copying
// first avoids race conditions with the running Slack process.
// Returns an error if no workspaces are discovered.
func ExtractWorkspaceTokens(leveldbDir string) ([]WorkspaceToken, error) {
	tmpDir, db, err := snapshotLevelDB(leveldbDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	defer db.Close()

	payload, err := findLocalConfigPayload(db, leveldbDir)
	if err != nil {
		return nil, err
	}
	return buildWorkspaceList(payload, leveldbDir)
}

// decodeLocalStorageValue decodes a Chromium LevelDB Local Storage value.
// Values are prefixed with a type byte:
//   - 0x01: UTF-8 string — strip the prefix byte.
//   - 0x00: UTF-16LE encoded string — convert to UTF-8.
func decodeLocalStorageValue(v []byte) ([]byte, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	switch v[0] {
	case 0x01: // UTF-8
		return v[1:], nil
	case 0x00: // UTF-16LE
		raw := v[1:]
		if len(raw)%2 != 0 {
			return nil, fmt.Errorf("UTF-16 value has odd byte count %d", len(raw))
		}
		u16 := make([]uint16, len(raw)/2)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
		return []byte(string(utf16.Decode(u16))), nil
	default:
		return nil, fmt.Errorf("unknown Local Storage value prefix 0x%02x", v[0])
	}
}

