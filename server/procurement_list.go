package main

import (
	"time"

	"gorm.io/gorm"
)

// Explicit DTOs keep contact details, specifications, attachments and histories out of lists.
type procRevisionSummary struct {
	ID             uint64    `json:"id"`
	Status         string    `json:"status"`
	Title          string    `json:"title"`
	DeliveryRegion string    `json:"delivery_region"`
	Deadline       time.Time `json:"deadline"`
}
type procItemSummary struct {
	ID         uint64 `json:"id"`
	CategoryID uint64 `json:"category_id"`
	Name       string `json:"name"`
	Quantity   string `json:"quantity"`
	Unit       string `json:"unit"`
}
type procParticipationSummary struct {
	State  string `json:"state"`
	Intent bool   `json:"intent"`
}
type procProjectSummary struct {
	ID          uint64                    `json:"id"`
	HallID      uint64                    `json:"hall_id"`
	State       string                    `json:"state"`
	Audience    string                    `json:"audience"`
	Revision    procRevisionSummary       `json:"revision"`
	Items       []procItemSummary         `json:"items"`
	Participant *procParticipationSummary `json:"participant,omitempty"`
}

func (a *App) procProjectSummaries(who Account, projects []ProcurementProject) ([]procProjectSummary, error) {
	result := make([]procProjectSummary, 0, len(projects))
	if len(projects) == 0 {
		return result, nil
	}
	ids := make([]uint64, 0, len(projects))
	revisionIDs := make([]uint64, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ID)
		id := p.PublishedRevisionID
		if id == 0 && (p.BuyerID == who.ID || platformAccount(who)) {
			id = p.DraftRevisionID
		}
		revisionIDs = append(revisionIDs, id)
	}
	var revisions []ProcurementRevision
	if err := a.DB.Select("id,project_id,status,title,delivery_region,deadline").Where("id IN ?", revisionIDs).Find(&revisions).Error; err != nil {
		return nil, err
	}
	revs := make(map[uint64]ProcurementRevision, len(revisions))
	for _, r := range revisions {
		revs[r.ID] = r
	}
	var lines []ProcurementItem
	if err := a.DB.Select("id,revision_id,category_id,name,quantity,unit").Where("revision_id IN ?", revisionIDs).Order("id").Find(&lines).Error; err != nil {
		return nil, err
	}
	items := make(map[uint64][]procItemSummary)
	for _, i := range lines {
		items[i.RevisionID] = append(items[i.RevisionID], procItemSummary{i.ID, i.CategoryID, i.Name, i.Quantity, i.Unit})
	}
	participants := make(map[uint64]ProcurementParticipant)
	if !platformAccount(who) {
		var own []ProcurementParticipant
		if err := a.DB.Table("procurement_participants pp").Select("pp.id,pp.project_id,pp.state,pp.intent").Joins("JOIN contents v ON v.id=pp.vendor_id AND v.kind='vendor'").Where("pp.project_id IN ? AND v.owner_id=? AND pp.state<>'excluded'", ids, who.ID).Order("pp.id").Scan(&own).Error; err != nil {
			return nil, err
		}
		for _, p := range own {
			if _, exists := participants[p.ProjectID]; !exists {
				participants[p.ProjectID] = p
			}
		}
	}
	for _, p := range projects {
		owner := p.BuyerID == who.ID || platformAccount(who)
		part, invited := participants[p.ID]
		if !owner && !invited {
			return nil, gorm.ErrRecordNotFound
		}
		id := p.PublishedRevisionID
		if id == 0 && owner {
			id = p.DraftRevisionID
		}
		r, exists := revs[id]
		if !exists || r.ProjectID != p.ID {
			return nil, gorm.ErrRecordNotFound
		}
		list := items[id]
		if list == nil {
			list = []procItemSummary{}
		}
		summary := procProjectSummary{ID: p.ID, HallID: p.HallID, State: procDisplayState(p, r), Audience: p.Audience, Revision: procRevisionSummary{r.ID, r.Status, r.Title, r.DeliveryRegion, r.Deadline}, Items: list}
		if !owner {
			summary.Participant = &procParticipationSummary{part.State, part.Intent}
		}
		result = append(result, summary)
	}
	return result, nil
}
