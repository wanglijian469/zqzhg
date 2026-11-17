package main

import (
	"encoding/json"
	"time"
)

type AIImportWork struct {
	Product      string    `json:"product,omitempty"`
	ProductIndex int       `json:"product_index,omitempty"`
	ProductTotal int       `json:"product_total,omitempty"`
	SourcePage   int       `json:"source_page,omitempty"`
	Done         int       `json:"done"`
	Total        int       `json:"total"`
	Message      string    `json:"message"`
	StartedAt    time.Time `json:"started_at"`
}

func aiProductWorkUnits(product AIInventoryProduct) int {
	units := 2*len(product.SourcePages) + 1
	if len(product.SourcePages) > 1 {
		units++
	}
	return units
}

func (a *App) aiWork(task *AIImportTask, stage string, progress int, work AIImportWork) error {
	work.StartedAt = time.Now().UTC()
	c := a.aiReadCheckpoint(task.ID)
	c.Work = &work
	if e := a.DB.Model(&AIImportTask{}).Where("id=? AND lease=? AND status='processing'", task.ID, task.Lease).Update("checkpoint", aiJSON(c)).Error; e != nil {
		return e
	}
	return a.aiStage(task, stage, progress)
}

func (task AIImportTask) MarshalJSON() ([]byte, error) {
	type plain AIImportTask
	var c aiImportCheckpoint
	_ = json.Unmarshal([]byte(task.Checkpoint), &c)
	return json.Marshal(struct {
		plain
		Work *AIImportWork `json:"work,omitempty"`
	}{plain(task), c.Work})
}
