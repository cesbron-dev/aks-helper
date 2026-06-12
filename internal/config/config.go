// Package config manages the on-disk layout of aks-helper.
//
// Everything lives under ~/.kube/aks:
//
//	~/.kube/aks/<name>.yaml   one standalone kubeconfig per stored cluster
//	~/.kube/aks/index.json    metadata (subscription / resource group / ...)
//	~/.kube/aks/.current      name of the currently selected cluster
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry holds the metadata we keep about a stored cluster so that `list` can
// display rich information without re-querying Azure.
type Entry struct {
	Name           string    `json:"name"`
	SubscriptionID string    `json:"subscriptionId"`
	Subscription   string    `json:"subscription"`
	ResourceGroup  string    `json:"resourceGroup"`
	ClusterName    string    `json:"clusterName"`
	LoginMode      string    `json:"loginMode"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Store is the directory-backed collection of stored clusters.
type Store struct {
	Dir string
}

// Default returns the store rooted at ~/.kube/aks, honouring the AKS_HELPER_DIR
// override if set.
func Default() (*Store, error) {
	if dir := os.Getenv("AKS_HELPER_DIR"); dir != "" {
		return New(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return New(filepath.Join(home, ".kube", "aks"))
}

// New returns a store rooted at dir, creating it if necessary.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// Path returns the kubeconfig path for a stored cluster name.
func (s *Store) Path(name string) string {
	return filepath.Join(s.Dir, name+".yaml")
}

func (s *Store) indexPath() string   { return filepath.Join(s.Dir, "index.json") }
func (s *Store) currentPath() string { return filepath.Join(s.Dir, ".current") }

// Exists reports whether a cluster with the given name is stored.
func (s *Store) Exists(name string) bool {
	_, err := os.Stat(s.Path(name))
	return err == nil
}

// List returns every stored cluster, sorted by name. Entries missing from the
// index are still returned (with whatever metadata could be inferred) so that
// kubeconfig files dropped in manually are not hidden.
func (s *Store) List() ([]Entry, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, p := range matches {
		name := strings.TrimSuffix(filepath.Base(p), ".yaml")
		if e, ok := index[name]; ok {
			entries = append(entries, e)
		} else {
			entries = append(entries, Entry{Name: name})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// Names returns the sorted names of every stored cluster.
func (s *Store) Names() ([]string, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// Get returns the metadata entry for a stored cluster.
func (s *Store) Get(name string) (Entry, bool, error) {
	index, err := s.loadIndex()
	if err != nil {
		return Entry{}, false, err
	}
	e, ok := index[name]
	return e, ok, nil
}

// Save records (or updates) an entry's metadata in the index.
func (s *Store) Save(e Entry) error {
	index, err := s.loadIndex()
	if err != nil {
		return err
	}
	e.UpdatedAt = time.Now()
	index[e.Name] = e
	return s.saveIndex(index)
}

// Remove deletes a stored cluster's kubeconfig and its index entry, and clears
// the current selection if it pointed at the removed cluster.
func (s *Store) Remove(name string) error {
	if err := os.Remove(s.Path(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	index, err := s.loadIndex()
	if err != nil {
		return err
	}
	delete(index, name)
	if err := s.saveIndex(index); err != nil {
		return err
	}
	if cur, _ := s.Current(); cur == name {
		_ = os.Remove(s.currentPath())
	}
	return nil
}

// SetCurrent records the currently selected cluster name.
func (s *Store) SetCurrent(name string) error {
	return os.WriteFile(s.currentPath(), []byte(name+"\n"), 0o600)
}

// Current returns the currently selected cluster name, or "" if none.
func (s *Store) Current() (string, error) {
	data, err := os.ReadFile(s.currentPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Store) loadIndex() (map[string]Entry, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Entry{}, nil
		}
		return nil, err
	}
	index := map[string]Entry{}
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing index.json: %w", err)
	}
	return index, nil
}

func (s *Store) saveIndex(index map[string]Entry) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.indexPath(), data, 0o600)
}
