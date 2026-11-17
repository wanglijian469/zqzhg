package main

import "sort"

type AIInventory struct {
	Products   []AIInventoryProduct `json:"products"`
	Unassigned []string             `json:"unassigned_support_blocks"`
	Issues     []string             `json:"review_issues"`
}

// Independently presented product blocks must not disappear merely because
// their titles/model sets match. Support-only continuations may be shared;
// multiple primary photo blocks remain separate drafts for human merge review.
func aiPreserveIndependentProducts(index map[int]AIPageIndex, in AIInventory) AIInventory {
	blocks := map[string]AIPageBlock{}
	pageOf := map[string]int{}
	primaries := map[int][]string{}
	for page, entry := range index {
		for _, b := range entry.Blocks {
			blocks[b.BlockID] = b
			pageOf[b.BlockID] = page
			if b.Kind == "product" {
				primaries[page] = append(primaries[page], b.BlockID)
			}
		}
	}
	out := AIInventory{Issues: in.Issues}
	position := map[string]int{}
	add := func(id string, original AIInventoryProduct) {
		at, exists := position[id]
		if !exists {
			b := blocks[id]
			name := original.ProductName
			if name == "" {
				name = b.Title
			}
			out.Products = append(out.Products, AIInventoryProduct{ProductName: name, SeriesName: b.SeriesName, BlockIDs: []string{id}})
			at = len(out.Products) - 1
			position[id] = at
		}
		for _, support := range original.BlockIDs {
			b, ok := blocks[support]
			if ok && b.Kind != "product" && (pageOf[support] == pageOf[id] || len(primaries[pageOf[support]]) == 0) {
				out.Products[at].BlockIDs = aiUniqueStrings(append(out.Products[at].BlockIDs, support))
			}
		}
	}
	for _, p := range in.Products {
		ids := []string{}
		for _, id := range p.BlockIDs {
			if b, ok := blocks[id]; ok && b.Kind == "product" {
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			q := p
			if len(ids) > 1 {
				q.ProductName = blocks[id].Title
			}
			add(id, q)
		}
		if len(ids) > 1 {
			out.Issues = append(out.Issues, "多个独立展示区块曾被目录合并；现按各区块保留草稿，请按结构和配置核对，确定重复后可人工合并。")
		}
	}
	pages := make([]int, 0, len(index))
	for p := range index {
		pages = append(pages, p)
	}
	sort.Ints(pages)
	for _, page := range pages {
		for _, b := range index[page].Blocks {
			if b.Kind == "product" {
				if _, ok := position[b.BlockID]; !ok {
					add(b.BlockID, AIInventoryProduct{})
					out.Issues = append(out.Issues, "已补回遗漏的独立产品区块："+b.Title)
				}
			}
		}
	}
	for _, page := range pages {
		if len(primaries[page]) == 1 {
			at := position[primaries[page][0]]
			for _, b := range index[page].Blocks {
				if b.Kind == "support" {
					out.Products[at].BlockIDs = aiUniqueStrings(append(out.Products[at].BlockIDs, b.BlockID))
				}
			}
		}
	}
	covered := map[string]bool{}
	for _, p := range out.Products {
		for _, id := range p.BlockIDs {
			covered[id] = true
		}
	}
	for _, page := range pages {
		for _, b := range index[page].Blocks {
			if b.Kind == "support" && !covered[b.BlockID] {
				out.Unassigned = append(out.Unassigned, b.BlockID)
			}
		}
	}
	sort.SliceStable(out.Products, func(i, j int) bool { return pageOf[out.Products[i].BlockIDs[0]] < pageOf[out.Products[j].BlockIDs[0]] })
	return out
}

func aiUniqueStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
