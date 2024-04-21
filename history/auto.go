package history

import (
	"fmt"
	"os"
	"time"

	"github.com/ChaosNyaruko/ondict/util"
)

func Append(word string) error {
	// TODO: log rotation to avoid too-big files
	t := util.HistoryTable()
	table, err := os.OpenFile(t, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s err: %v", t, err)
	}
	if _, err := table.WriteString(fmt.Sprintf("%s | %s\n", time.Now(), word)); err != nil {
		return fmt.Errorf("write a record error: %v", err)
	}
	defer table.Close()

	return nil
}
