package jobkit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/stringutil"
)

// HistoryJSON is a history provider that writes history as a json object to disk.
type HistoryJSON struct {
	Config JobConfig
}

// PersistHistory writes the history to disk fully, overwriting any existing file.
func (hj HistoryJSON) PersistHistory(ctx context.Context, history []*JobInvocation) error {
	historyDirectory := hj.Config.HistoryPathOrDefault()
	if _, err := os.Stat(historyDirectory); err != nil {
		if err := os.MkdirAll(historyDirectory, 0755); err != nil {
			return ex.New(err)
		}
	}
	historyPath := filepath.Join(historyDirectory, stringutil.Slugify(hj.Config.Name)+".json")
	f, err := os.Create(historyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(history)
}

// RestoreHistory reads history from disk and returns the log.
func (hj HistoryJSON) RestoreHistory(ctx context.Context) (output []*JobInvocation, err error) {
	historyPath := filepath.Join(hj.Config.HistoryPathOrDefault(), stringutil.Slugify(hj.Config.Name)+".json")
	if _, statErr := os.Stat(historyPath); statErr != nil {
		return
	}
	var f *os.File
	f, err = os.Open(historyPath)
	if err != nil {
		return
	}
	defer f.Close()

	var intermediate []JobInvocation
	if err = ex.New(json.NewDecoder(f).Decode(&intermediate)); err != nil {
		return
	}
	output = make([]*JobInvocation, len(intermediate))
	for index := range intermediate {
		output[index] = &intermediate[index]
	}
	return
}
